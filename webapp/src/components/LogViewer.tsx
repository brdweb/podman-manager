import { useEffect, useRef, useState, useCallback } from 'react';
import { websocketURL } from '../api/client';

interface LogViewerProps {
  host: string;
  containerId: string;
  containerName: string;
  onClose: () => void;
}

interface LogStreamMessage {
  timestamp?: string;
  message?: string;
  error?: string;
}

const TAIL_OPTIONS = [100, 500, 1000, 5000] as const;

export function LogViewer({ host, containerId, containerName, onClose }: LogViewerProps) {
  const [tail, setTail] = useState<number>(100);
  const [logs, setLogs] = useState<string>('');
  const [isPaused, setIsPaused] = useState(false);
  const [status, setStatus] = useState<'connecting' | 'connected' | 'closed' | 'error'>('connecting');
  const [error, setError] = useState<string | null>(null);

  const logsEndRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const pausedRef = useRef(false);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    pausedRef.current = isPaused;
  }, [isPaused]);

  useEffect(() => {
    const url = websocketURL(
      `/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(containerId)}/logs/stream?tail=${tail}`
    );
    const ws = new WebSocket(url);

    ws.addEventListener('open', () => {
      setLogs('');
      setError(null);
      setStatus('connected');
    });

    ws.addEventListener('message', (event) => {
      let payload: LogStreamMessage;
      try {
        payload = JSON.parse(String(event.data)) as LogStreamMessage;
      } catch {
        payload = { message: String(event.data) };
      }

      if (payload.error) {
        setError(payload.error);
        setStatus('error');
        return;
      }

      if (pausedRef.current || !payload.message) {
        return;
      }

      setLogs((current) => `${current}${current ? '\n' : ''}${payload.message}`);
    });

    ws.addEventListener('error', () => {
      setError('Log stream connection failed');
      setStatus('error');
    });

    ws.addEventListener('close', () => {
      setStatus((current) => (current === 'error' ? current : 'closed'));
    });

    return () => {
      ws.close();
    };
  }, [containerId, host, tail]);

  const isLoading = status === 'connecting' && logs === '' && !error;

  useEffect(() => {
    if (autoScroll && logsEndRef.current && !isLoading) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [autoScroll, isLoading, logs]);

  const handleScroll = useCallback(() => {
    if (!scrollContainerRef.current) return;
    
    const { scrollTop, scrollHeight, clientHeight } = scrollContainerRef.current;
    const isAtBottom = Math.abs(scrollHeight - clientHeight - scrollTop) < 10;
    
    if (!isAtBottom && autoScroll) {
      setAutoScroll(false);
    } else if (isAtBottom && !autoScroll) {
      setAutoScroll(true);
    }
  }, [autoScroll]);

  const handleLoadMore = () => {
    const currentIndex = TAIL_OPTIONS.indexOf(tail as typeof TAIL_OPTIONS[number]);
    if (currentIndex < TAIL_OPTIONS.length - 1) {
      setTail(TAIL_OPTIONS[currentIndex + 1]);
    }
  };

  const canLoadMore = tail !== TAIL_OPTIONS[TAIL_OPTIONS.length - 1];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm">
      <div className="flex h-full max-h-[80vh] w-full max-w-5xl flex-col overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950 shadow-2xl">
        <div className="flex items-center justify-between border-b border-zinc-800 bg-zinc-900/50 px-4 py-3">
          <div className="flex items-center gap-3">
            <h3 className="font-medium text-zinc-100">Logs: {containerName}</h3>
            <span className="text-xs text-zinc-500">Last {tail} lines</span>
            <span className="rounded-full border border-zinc-800 px-2 py-0.5 text-[10px] uppercase tracking-[0.18em] text-zinc-500">
              {status}
            </span>
          </div>
          
          <div className="flex items-center gap-2">
            <select
              value={tail}
              onChange={(e) => setTail(Number(e.target.value))}
              className="rounded bg-zinc-800 px-2 py-1 text-xs text-zinc-200 outline-none"
            >
              {TAIL_OPTIONS.map((opt) => (
                <option key={opt} value={opt}>{opt} lines</option>
              ))}
            </select>
            <button
              type="button"
              onClick={() => setAutoScroll(!autoScroll)}
              className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
                autoScroll ? 'bg-zinc-800 text-zinc-200' : 'bg-zinc-900 text-zinc-500 hover:bg-zinc-800 hover:text-zinc-300'
              }`}
            >
              Auto-scroll {autoScroll ? 'On' : 'Off'}
            </button>
            <button
              type="button"
              onClick={() => setIsPaused(!isPaused)}
              className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
                isPaused ? 'bg-orange-600 text-white hover:bg-orange-500' : 'bg-zinc-800 text-zinc-200 hover:bg-zinc-700'
              }`}
            >
              {isPaused ? 'Resume' : 'Pause'}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="ml-2 rounded p-1.5 text-zinc-400 hover:bg-zinc-800 hover:text-white transition-colors"
              aria-label="Close"
            >
              <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <title>Close</title>
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>
        </div>

        <div 
          ref={scrollContainerRef}
          onScroll={handleScroll}
          className="flex-1 overflow-y-auto p-4 font-mono text-xs text-zinc-300"
        >
          {isLoading ? (
            <div className="flex h-full items-center justify-center text-zinc-500">
              Loading logs...
            </div>
          ) : error ? (
            <div className="flex h-full items-center justify-center text-red-400">
              {error}
            </div>
          ) : (
            <div className="space-y-0.5">
              <pre className="whitespace-pre-wrap break-all">{logs || 'Waiting for log output...'}</pre>
              <div ref={logsEndRef} />
            </div>
          )}
        </div>

        {!isLoading && !error && canLoadMore && (
          <div className="border-t border-zinc-800 bg-zinc-900/50 px-4 py-2">
            <button
              type="button"
              onClick={handleLoadMore}
              className="w-full rounded bg-zinc-800 py-2 text-xs font-medium text-zinc-300 hover:bg-zinc-700 transition-colors"
            >
              Load More (next: {TAIL_OPTIONS[TAIL_OPTIONS.indexOf(tail as typeof TAIL_OPTIONS[number]) + 1]} lines)
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
