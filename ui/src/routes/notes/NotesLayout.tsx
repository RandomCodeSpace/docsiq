import { Outlet } from "react-router-dom";

export default function NotesLayout() {
  return (
    <div className="p-6">
      <h1 className="text-xl font-semibold">Notes</h1>
      <p className="text-sm text-[var(--color-text-muted)] mt-2">Stub — implemented in Wave 5.</p>
      <Outlet />
    </div>
  );
}
