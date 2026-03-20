package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the single SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path and runs migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite WAL allows 1 writer
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// Idempotent column additions for existing databases
	migrations := []string{
		`ALTER TABLE documents ADD COLUMN version      INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE documents ADD COLUMN canonical_id TEXT`,
		`ALTER TABLE documents ADD COLUMN is_latest    INTEGER NOT NULL DEFAULT 1`,
	}
	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("migration failed: %w", err)
			}
		}
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS documents (
    id           TEXT PRIMARY KEY,
    path         TEXT NOT NULL,
    title        TEXT,
    doc_type     TEXT,
    file_hash    TEXT UNIQUE,
    structured   TEXT,
    version      INTEGER NOT NULL DEFAULT 1,
    canonical_id TEXT,              -- NULL on first version; points to v1 ID on all later versions
    is_latest    INTEGER NOT NULL DEFAULT 1,
    created_at   INTEGER,
    updated_at   INTEGER
);

CREATE TABLE IF NOT EXISTS chunks (
    id          TEXT PRIMARY KEY,
    doc_id      TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content     TEXT NOT NULL,
    token_count INTEGER,
    metadata    TEXT
);

CREATE TABLE IF NOT EXISTS embeddings (
    chunk_id    TEXT PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    model       TEXT NOT NULL,
    dims        INTEGER NOT NULL,
    vector      BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS entities (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    type         TEXT,
    description  TEXT,
    rank         INTEGER DEFAULT 0,
    community_id TEXT,
    vector       BLOB
);

CREATE TABLE IF NOT EXISTS relationships (
    id           TEXT PRIMARY KEY,
    source_id    TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    target_id    TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    predicate    TEXT NOT NULL,
    description  TEXT,
    weight       REAL DEFAULT 1.0,
    doc_id       TEXT REFERENCES documents(id)
);

CREATE TABLE IF NOT EXISTS claims (
    id        TEXT PRIMARY KEY,
    entity_id TEXT REFERENCES entities(id),
    claim     TEXT NOT NULL,
    status    TEXT,
    doc_id    TEXT REFERENCES documents(id)
);

CREATE TABLE IF NOT EXISTS communities (
    id        TEXT PRIMARY KEY,
    level     INTEGER NOT NULL,
    parent_id TEXT REFERENCES communities(id),
    title     TEXT,
    summary   TEXT,
    rank      INTEGER DEFAULT 0,
    vector    BLOB
);

CREATE TABLE IF NOT EXISTS community_members (
    community_id TEXT NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    entity_id    TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    PRIMARY KEY (community_id, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_doc_canonical      ON documents(canonical_id);
CREATE INDEX IF NOT EXISTS idx_doc_path_latest    ON documents(path, is_latest);
CREATE INDEX IF NOT EXISTS idx_chunks_doc         ON chunks(doc_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_model   ON embeddings(model);
CREATE INDEX IF NOT EXISTS idx_entities_name      ON entities(name);
CREATE INDEX IF NOT EXISTS idx_entities_community ON entities(community_id);
CREATE INDEX IF NOT EXISTS idx_rel_source         ON relationships(source_id);
CREATE INDEX IF NOT EXISTS idx_rel_target         ON relationships(target_id);
CREATE INDEX IF NOT EXISTS idx_claims_entity      ON claims(entity_id);
CREATE INDEX IF NOT EXISTS idx_comm_level         ON communities(level);
`

// ── float32 vector encoding ───────────────────────────────────────────────────

func EncodeVector(v []float32) []byte {
	b := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

func DecodeVector(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// ── Document CRUD ─────────────────────────────────────────────────────────────

type Document struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	DocType     string `json:"doc_type"`
	FileHash    string `json:"file_hash,omitempty"`
	Structured  string `json:"structured,omitempty"`
	Version     int    `json:"version,omitempty"`
	CanonicalID string `json:"canonical_id,omitempty"` // empty on v1; ID of first version on v2+
	IsLatest    bool   `json:"is_latest,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// CanonicalOrID returns CanonicalID if set, otherwise the document's own ID.
// This is the stable identifier across all versions of a file.
func (d *Document) CanonicalOrID() string {
	if d.CanonicalID != "" {
		return d.CanonicalID
	}
	return d.ID
}

func (s *Store) UpsertDocument(ctx context.Context, doc *Document) error {
	now := time.Now().Unix()
	if doc.Version == 0 {
		doc.Version = 1
	}
	var canonicalID any
	if doc.CanonicalID != "" {
		canonicalID = doc.CanonicalID
	}
	isLatest := 0
	if doc.IsLatest {
		isLatest = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO documents (id, path, title, doc_type, file_hash, structured, version, canonical_id, is_latest, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path, title=excluded.title, doc_type=excluded.doc_type,
			file_hash=excluded.file_hash, structured=excluded.structured,
			version=excluded.version, canonical_id=excluded.canonical_id,
			is_latest=excluded.is_latest, updated_at=excluded.updated_at`,
		doc.ID, doc.Path, doc.Title, doc.DocType, doc.FileHash, doc.Structured,
		doc.Version, canonicalID, isLatest, now, now)
	return err
}

// SupersedeDocument marks a document as no longer the latest version.
func (s *Store) SupersedeDocument(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE documents SET is_latest=0, updated_at=? WHERE id=?`, time.Now().Unix(), id)
	return err
}

// GetDocumentVersions returns all versions of a document by canonical ID, oldest first.
func (s *Store) GetDocumentVersions(ctx context.Context, canonicalID string) ([]*Document, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,path,title,doc_type,file_hash,structured,version,canonical_id,is_latest,created_at,updated_at
		FROM documents
		WHERE id=? OR canonical_id=?
		ORDER BY version ASC`, canonicalID, canonicalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []*Document
	for rows.Next() {
		d, err := scanDocRow(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

const docSelect = `SELECT id,path,title,doc_type,file_hash,structured,version,canonical_id,is_latest,created_at,updated_at FROM documents`

func scanDocRow(rows *sql.Rows) (*Document, error) {
	var d Document
	var canonicalID sql.NullString
	var isLatest int
	err := rows.Scan(&d.ID, &d.Path, &d.Title, &d.DocType, &d.FileHash, &d.Structured,
		&d.Version, &canonicalID, &isLatest, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if canonicalID.Valid {
		d.CanonicalID = canonicalID.String
	}
	d.IsLatest = isLatest == 1
	return &d, nil
}

func scanDocSingleRow(row *sql.Row) (*Document, error) {
	var d Document
	var canonicalID sql.NullString
	var isLatest int
	err := row.Scan(&d.ID, &d.Path, &d.Title, &d.DocType, &d.FileHash, &d.Structured,
		&d.Version, &canonicalID, &isLatest, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if canonicalID.Valid {
		d.CanonicalID = canonicalID.String
	}
	d.IsLatest = isLatest == 1
	return &d, nil
}

func (s *Store) GetDocumentByHash(ctx context.Context, hash string) (*Document, error) {
	return scanDocSingleRow(s.db.QueryRowContext(ctx, docSelect+` WHERE file_hash=?`, hash))
}

func (s *Store) GetDocumentByPath(ctx context.Context, path string) (*Document, error) {
	// Returns the latest version at this path.
	return scanDocSingleRow(s.db.QueryRowContext(ctx, docSelect+` WHERE path=? AND is_latest=1`, path))
}

func (s *Store) GetDocument(ctx context.Context, id string) (*Document, error) {
	return scanDocSingleRow(s.db.QueryRowContext(ctx, docSelect+` WHERE id=?`, id))
}

func (s *Store) ListDocuments(ctx context.Context, docType string, limit, offset int) ([]*Document, error) {
	// Default: only latest versions.
	q := docSelect + ` WHERE is_latest=1`
	args := []any{}
	if docType != "" {
		q += ` AND doc_type=?`
		args = append(args, docType)
	}
	q += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []*Document
	for rows.Next() {
		d, err := scanDocRow(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

func (s *Store) UpdateDocumentStructured(ctx context.Context, id, structured string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE documents SET structured=?, updated_at=? WHERE id=?`, structured, time.Now().Unix(), id)
	return err
}

// ── Chunk CRUD ────────────────────────────────────────────────────────────────

type Chunk struct {
	ID         string
	DocID      string
	ChunkIndex int
	Content    string
	TokenCount int
	Metadata   string
}

func (s *Store) InsertChunk(ctx context.Context, c *Chunk) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO chunks (id, doc_id, chunk_index, content, token_count, metadata) VALUES (?,?,?,?,?,?)`,
		c.ID, c.DocID, c.ChunkIndex, c.Content, c.TokenCount, c.Metadata)
	return err
}

func (s *Store) GetChunk(ctx context.Context, id string) (*Chunk, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,doc_id,chunk_index,content,token_count,metadata FROM chunks WHERE id=?`, id)
	var c Chunk
	err := row.Scan(&c.ID, &c.DocID, &c.ChunkIndex, &c.Content, &c.TokenCount, &c.Metadata)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (s *Store) ListChunksByDoc(ctx context.Context, docID string) ([]*Chunk, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,doc_id,chunk_index,content,token_count,metadata FROM chunks WHERE doc_id=? ORDER BY chunk_index`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chunks []*Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.ID, &c.DocID, &c.ChunkIndex, &c.Content, &c.TokenCount, &c.Metadata); err != nil {
			return nil, err
		}
		chunks = append(chunks, &c)
	}
	return chunks, rows.Err()
}

// ── Embedding CRUD ────────────────────────────────────────────────────────────

func (s *Store) UpsertEmbedding(ctx context.Context, chunkID, model string, vector []float32) error {
	blob := EncodeVector(vector)
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO embeddings (chunk_id, model, dims, vector) VALUES (?,?,?,?)`,
		chunkID, model, len(vector), blob)
	return err
}

type ChunkWithEmbedding struct {
	Chunk  Chunk
	Vector []float32
}

func (s *Store) AllChunkEmbeddings(ctx context.Context, model string) ([]ChunkWithEmbedding, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.doc_id, c.chunk_index, c.content, c.token_count, c.metadata, e.vector
		FROM chunks c JOIN embeddings e ON c.id=e.chunk_id WHERE e.model=?`, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []ChunkWithEmbedding
	for rows.Next() {
		var cwe ChunkWithEmbedding
		var blob []byte
		if err := rows.Scan(&cwe.Chunk.ID, &cwe.Chunk.DocID, &cwe.Chunk.ChunkIndex,
			&cwe.Chunk.Content, &cwe.Chunk.TokenCount, &cwe.Chunk.Metadata, &blob); err != nil {
			return nil, err
		}
		cwe.Vector = DecodeVector(blob)
		results = append(results, cwe)
	}
	return results, rows.Err()
}

// ── Entity CRUD ───────────────────────────────────────────────────────────────

type Entity struct {
	ID          string
	Name        string
	Type        string
	Description string
	Rank        int
	CommunityID string
	Vector      []float32
}

func (s *Store) UpsertEntity(ctx context.Context, e *Entity) error {
	var blob []byte
	if e.Vector != nil {
		blob = EncodeVector(e.Vector)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO entities (id, name, type, description, rank, community_id, vector)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, type=excluded.type, description=excluded.description,
			rank=excluded.rank, community_id=excluded.community_id, vector=excluded.vector`,
		e.ID, e.Name, e.Type, e.Description, e.Rank, e.CommunityID, blob)
	return err
}

func (s *Store) GetEntityByName(ctx context.Context, name string) (*Entity, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,type,description,rank,community_id,vector FROM entities WHERE name=? COLLATE NOCASE LIMIT 1`, name)
	return scanEntity(row)
}

func (s *Store) GetEntity(ctx context.Context, id string) (*Entity, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,type,description,rank,community_id,vector FROM entities WHERE id=?`, id)
	return scanEntity(row)
}

func (s *Store) ListEntities(ctx context.Context, typ string, limit, offset int) ([]*Entity, error) {
	q := `SELECT id,name,type,description,rank,community_id,vector FROM entities`
	args := []any{}
	if typ != "" {
		q += ` WHERE type=?`
		args = append(args, typ)
	}
	q += ` ORDER BY rank DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entities []*Entity
	for rows.Next() {
		e, err := scanEntityRow(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

func (s *Store) AllEntities(ctx context.Context) ([]*Entity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,type,description,rank,community_id,vector FROM entities`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entities []*Entity
	for rows.Next() {
		e, err := scanEntityRow(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

func (s *Store) UpdateEntityCommunity(ctx context.Context, entityID, communityID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE entities SET community_id=? WHERE id=?`, communityID, entityID)
	return err
}

func (s *Store) UpdateEntityRank(ctx context.Context, entityID string, rank int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE entities SET rank=? WHERE id=?`, rank, entityID)
	return err
}

func scanEntity(row *sql.Row) (*Entity, error) {
	var e Entity
	var blob []byte
	var communityID sql.NullString
	err := row.Scan(&e.ID, &e.Name, &e.Type, &e.Description, &e.Rank, &communityID, &blob)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if communityID.Valid {
		e.CommunityID = communityID.String
	}
	if blob != nil {
		e.Vector = DecodeVector(blob)
	}
	return &e, nil
}

func scanEntityRow(rows *sql.Rows) (*Entity, error) {
	var e Entity
	var blob []byte
	var communityID sql.NullString
	err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Description, &e.Rank, &communityID, &blob)
	if err != nil {
		return nil, err
	}
	if communityID.Valid {
		e.CommunityID = communityID.String
	}
	if blob != nil {
		e.Vector = DecodeVector(blob)
	}
	return &e, nil
}

// ── Relationship CRUD ─────────────────────────────────────────────────────────

type Relationship struct {
	ID          string
	SourceID    string
	TargetID    string
	Predicate   string
	Description string
	Weight      float64
	DocID       string
}

func (s *Store) InsertRelationship(ctx context.Context, r *Relationship) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO relationships (id, source_id, target_id, predicate, description, weight, doc_id) VALUES (?,?,?,?,?,?,?)`,
		r.ID, r.SourceID, r.TargetID, r.Predicate, r.Description, r.Weight, r.DocID)
	return err
}

func (s *Store) AllRelationships(ctx context.Context) ([]*Relationship, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_id,target_id,predicate,description,weight,doc_id FROM relationships`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rels []*Relationship
	for rows.Next() {
		var r Relationship
		var docID sql.NullString
		if err := rows.Scan(&r.ID, &r.SourceID, &r.TargetID, &r.Predicate, &r.Description, &r.Weight, &docID); err != nil {
			return nil, err
		}
		if docID.Valid {
			r.DocID = docID.String
		}
		rels = append(rels, &r)
	}
	return rels, rows.Err()
}

func (s *Store) RelationshipsForEntity(ctx context.Context, entityID string, depth int) ([]*Relationship, error) {
	// BFS up to depth levels
	visited := map[string]bool{entityID: true}
	frontier := []string{entityID}
	var all []*Relationship

	for d := 0; d < depth && len(frontier) > 0; d++ {
		placeholders := ""
		args := make([]any, len(frontier))
		for i, id := range frontier {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args[i] = id
		}
		q := fmt.Sprintf(`SELECT id,source_id,target_id,predicate,description,weight,doc_id FROM relationships WHERE source_id IN (%s) OR target_id IN (%s)`, placeholders, placeholders)
		rows, err := s.db.QueryContext(ctx, q, append(args, args...)...)
		if err != nil {
			return nil, err
		}
		var nextFrontier []string
		for rows.Next() {
			var r Relationship
			var docID sql.NullString
			if err := rows.Scan(&r.ID, &r.SourceID, &r.TargetID, &r.Predicate, &r.Description, &r.Weight, &docID); err != nil {
				rows.Close()
				return nil, err
			}
			if docID.Valid {
				r.DocID = docID.String
			}
			all = append(all, &r)
			for _, nid := range []string{r.SourceID, r.TargetID} {
				if !visited[nid] {
					visited[nid] = true
					nextFrontier = append(nextFrontier, nid)
				}
			}
		}
		rows.Close()
		frontier = nextFrontier
	}
	return all, nil
}

func (s *Store) FindRelationships(ctx context.Context, fromID, toID, predicate string) ([]*Relationship, error) {
	q := `SELECT id,source_id,target_id,predicate,description,weight,doc_id FROM relationships WHERE 1=1`
	args := []any{}
	if fromID != "" {
		q += ` AND source_id=?`
		args = append(args, fromID)
	}
	if toID != "" {
		q += ` AND target_id=?`
		args = append(args, toID)
	}
	if predicate != "" {
		q += ` AND predicate=?`
		args = append(args, predicate)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rels []*Relationship
	for rows.Next() {
		var r Relationship
		var docID sql.NullString
		if err := rows.Scan(&r.ID, &r.SourceID, &r.TargetID, &r.Predicate, &r.Description, &r.Weight, &docID); err != nil {
			return nil, err
		}
		if docID.Valid {
			r.DocID = docID.String
		}
		rels = append(rels, &r)
	}
	return rels, rows.Err()
}

// ── Claim CRUD ────────────────────────────────────────────────────────────────

type Claim struct {
	ID       string
	EntityID string
	Claim    string
	Status   string
	DocID    string
}

func (s *Store) InsertClaim(ctx context.Context, c *Claim) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO claims (id, entity_id, claim, status, doc_id) VALUES (?,?,?,?,?)`,
		c.ID, c.EntityID, c.Claim, c.Status, c.DocID)
	return err
}

// ── Community CRUD ────────────────────────────────────────────────────────────

type Community struct {
	ID       string
	Level    int
	ParentID string
	Title    string
	Summary  string
	Rank     int
	Vector   []float32
}

func (s *Store) UpsertCommunity(ctx context.Context, c *Community) error {
	var blob []byte
	if c.Vector != nil {
		blob = EncodeVector(c.Vector)
	}
	var parentID any
	if c.ParentID != "" {
		parentID = c.ParentID
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO communities (id, level, parent_id, title, summary, rank, vector)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			level=excluded.level, parent_id=excluded.parent_id, title=excluded.title,
			summary=excluded.summary, rank=excluded.rank, vector=excluded.vector`,
		c.ID, c.Level, parentID, c.Title, c.Summary, c.Rank, blob)
	return err
}

func (s *Store) InsertCommunityMember(ctx context.Context, communityID, entityID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO community_members (community_id, entity_id) VALUES (?,?)`,
		communityID, entityID)
	return err
}

func (s *Store) GetCommunity(ctx context.Context, id string) (*Community, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,level,parent_id,title,summary,rank,vector FROM communities WHERE id=?`, id)
	return scanCommunity(row)
}

func (s *Store) ListCommunities(ctx context.Context, level int) ([]*Community, error) {
	q := `SELECT id,level,parent_id,title,summary,rank,vector FROM communities`
	args := []any{}
	if level >= 0 {
		q += ` WHERE level=?`
		args = append(args, level)
	}
	q += ` ORDER BY rank DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var communities []*Community
	for rows.Next() {
		c, err := scanCommunityRow(rows)
		if err != nil {
			return nil, err
		}
		communities = append(communities, c)
	}
	return communities, rows.Err()
}

func (s *Store) AllCommunities(ctx context.Context) ([]*Community, error) {
	return s.ListCommunities(ctx, -1)
}

func (s *Store) CommunityMembers(ctx context.Context, communityID string) ([]*Entity, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id,e.name,e.type,e.description,e.rank,e.community_id,e.vector
		FROM entities e JOIN community_members cm ON e.id=cm.entity_id
		WHERE cm.community_id=?`, communityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entities []*Entity
	for rows.Next() {
		e, err := scanEntityRow(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

func (s *Store) ClearCommunities(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM community_members; DELETE FROM communities; UPDATE entities SET community_id=NULL`)
	return err
}

// GraphFingerprint returns counts of entities, relationships, and communities
// to detect whether the graph has changed since last finalization.
func (s *Store) GraphFingerprint(ctx context.Context) (entities, relationships, communities int, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM entities),
			(SELECT COUNT(*) FROM relationships),
			(SELECT COUNT(*) FROM communities)`).Scan(&entities, &relationships, &communities)
	return
}

func scanCommunity(row *sql.Row) (*Community, error) {
	var c Community
	var blob []byte
	var parentID sql.NullString
	err := row.Scan(&c.ID, &c.Level, &parentID, &c.Title, &c.Summary, &c.Rank, &blob)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		c.ParentID = parentID.String
	}
	if blob != nil {
		c.Vector = DecodeVector(blob)
	}
	return &c, nil
}

func scanCommunityRow(rows *sql.Rows) (*Community, error) {
	var c Community
	var blob []byte
	var parentID sql.NullString
	err := rows.Scan(&c.ID, &c.Level, &parentID, &c.Title, &c.Summary, &c.Rank, &blob)
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		c.ParentID = parentID.String
	}
	if blob != nil {
		c.Vector = DecodeVector(blob)
	}
	return &c, nil
}

// ── Stats ─────────────────────────────────────────────────────────────────────

type Stats struct {
	Documents     int `json:"documents"`
	Chunks        int `json:"chunks"`
	Embeddings    int `json:"embeddings"`
	Entities      int `json:"entities"`
	Relationships int `json:"relationships"`
	Claims        int `json:"claims"`
	Communities   int `json:"communities"`
}

func (s *Store) GetStats(ctx context.Context) (*Stats, error) {
	var st Stats
	row := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM documents),
			(SELECT COUNT(*) FROM chunks),
			(SELECT COUNT(*) FROM embeddings),
			(SELECT COUNT(*) FROM entities),
			(SELECT COUNT(*) FROM relationships),
			(SELECT COUNT(*) FROM claims),
			(SELECT COUNT(*) FROM communities)`)
	return &st, row.Scan(&st.Documents, &st.Chunks, &st.Embeddings, &st.Entities, &st.Relationships, &st.Claims, &st.Communities)
}

// ── Batch write helpers (single transaction) ──────────────────────────────────

// BatchInsertChunks inserts multiple chunks in one transaction.
func (s *Store) BatchInsertChunks(ctx context.Context, chunks []*Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO chunks (id, doc_id, chunk_index, content, token_count, metadata) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range chunks {
		if _, err := stmt.ExecContext(ctx, c.ID, c.DocID, c.ChunkIndex, c.Content, c.TokenCount, c.Metadata); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BatchUpsertEmbeddings upserts multiple embeddings in one transaction.
func (s *Store) BatchUpsertEmbeddings(ctx context.Context, model string, chunkIDs []string, vectors [][]float32) error {
	if len(chunkIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO embeddings (chunk_id, model, dims, vector) VALUES (?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, cid := range chunkIDs {
		if i >= len(vectors) {
			break
		}
		blob := EncodeVector(vectors[i])
		if _, err := stmt.ExecContext(ctx, cid, model, len(vectors[i]), blob); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BatchInsertRelationships inserts multiple relationships in one transaction.
func (s *Store) BatchInsertRelationships(ctx context.Context, rels []*Relationship) error {
	if len(rels) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO relationships (id, source_id, target_id, predicate, description, weight, doc_id) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rels {
		if _, err := stmt.ExecContext(ctx, r.ID, r.SourceID, r.TargetID, r.Predicate, r.Description, r.Weight, r.DocID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BatchInsertClaims inserts multiple claims in one transaction.
func (s *Store) BatchInsertClaims(ctx context.Context, claims []*Claim) error {
	if len(claims) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO claims (id, entity_id, claim, status, doc_id) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range claims {
		if _, err := stmt.ExecContext(ctx, c.ID, c.EntityID, c.Claim, c.Status, c.DocID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BatchUpsertEntities upserts multiple entities in one transaction.
func (s *Store) BatchUpsertEntities(ctx context.Context, entities []*Entity) error {
	if len(entities) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO entities (id, name, type, description, rank, community_id, vector)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, type=excluded.type, description=excluded.description,
			rank=excluded.rank, community_id=excluded.community_id, vector=excluded.vector`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, e := range entities {
		var blob []byte
		if e.Vector != nil {
			blob = EncodeVector(e.Vector)
		}
		var communityID any
		if e.CommunityID != "" {
			communityID = e.CommunityID
		}
		if _, err := stmt.ExecContext(ctx, e.ID, e.Name, e.Type, e.Description, e.Rank, communityID, blob); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BatchUpdateEntityCommunities updates community_id for multiple entities in one transaction.
func (s *Store) BatchUpdateEntityCommunities(ctx context.Context, assignments map[string]string) error {
	if len(assignments) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `UPDATE entities SET community_id=? WHERE id=?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for eid, cid := range assignments {
		if _, err := stmt.ExecContext(ctx, cid, eid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BatchUpdateEntityRanks updates rank for multiple entities in one transaction.
func (s *Store) BatchUpdateEntityRanks(ctx context.Context, ranks map[string]int) error {
	if len(ranks) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `UPDATE entities SET rank=? WHERE id=?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for eid, rank := range ranks {
		if _, err := stmt.ExecContext(ctx, rank, eid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BatchInsertCommunityMembers inserts community memberships in one transaction.
func (s *Store) BatchInsertCommunityMembers(ctx context.Context, communityID string, entityIDs []string) error {
	if len(entityIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO community_members (community_id, entity_id) VALUES (?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, eid := range entityIDs {
		if _, err := stmt.ExecContext(ctx, communityID, eid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetEntitiesByNames fetches entities for a set of names in one query. Returns map name→Entity.
func (s *Store) GetEntitiesByNames(ctx context.Context, names []string) (map[string]*Entity, error) {
	if len(names) == 0 {
		return map[string]*Entity{}, nil
	}
	// Build IN clause
	placeholders := make([]string, len(names))
	args := make([]any, len(names))
	for i, n := range names {
		placeholders[i] = "?"
		args[i] = n
	}
	q := `SELECT id,name,type,description,rank,community_id,vector FROM entities WHERE name IN (` +
		joinStrings(placeholders, ",") + `) COLLATE NOCASE`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]*Entity, len(names))
	for rows.Next() {
		e, err := scanEntityRow(rows)
		if err != nil {
			return nil, err
		}
		result[e.Name] = e
	}
	return result, rows.Err()
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
