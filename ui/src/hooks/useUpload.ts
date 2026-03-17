import { useRef, useState } from 'react'

export function useUpload() {
  const [jobId, setJobId] = useState('')
  const [status, setStatus] = useState('idle')
  const [messages, setMessages] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const sourceRef = useRef<EventSource | null>(null)

  const upload = async (files: File[]) => {
    if (!files.length) return
    setLoading(true)
    setStatus('uploading')
    setMessages([])

    const form = new FormData()
    for (const file of files) form.append('files', file)

    try {
      const res = await fetch('/api/upload', { method: 'POST', body: form })
      const data = await res.json()
      setJobId(data.job_id ?? '')
      setStatus(data.status ?? 'queued')

      sourceRef.current?.close()
      const source = new EventSource('/api/upload/progress')
      source.onmessage = (event) => {
        const text = event.data
        setMessages((prev) => [...prev, text])
        if (text === 'done') {
          setStatus('done')
          setLoading(false)
          source.close()
        }
        if (text.startsWith('error:')) {
          setStatus('error')
          setLoading(false)
          source.close()
        }
      }
      source.onerror = () => {
        source.close()
        setLoading(false)
      }
      sourceRef.current = source
    } catch (e) {
      setMessages([String(e)])
      setStatus('error')
      setLoading(false)
    }
  }

  return { jobId, status, messages, loading, upload }
}
