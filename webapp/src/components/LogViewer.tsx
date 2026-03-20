import { useEffect, useRef, useState, useCallback } from 'react';

interface LogViewerProps {
  host: string;
  containerId: string;
  containerName: string;
  onClose: () => void;
}

interface LogEntry {
  timestamp: string;
  message: string;
}

export function LogViewer({ host, containerId, containerName, onClose }: LogViewerProps) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [isPaused, setIsPaused] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting');
  
  const wsRef = useRef<WebSocket | null>(null);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const reconnectTimeoutRef = useRef<number | undefined>(undefined);
  const isPausedRef = useRef(isPaused);

  // Keep ref in sync with state for the websocket message handler
  useEffect(() => {
    isPausedRef.current = isPaused;
  }, [isPaused]);

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    setStatus('connecting');
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/hosts/${encodeURIComponent(host)}/containers/${encodeURIComponent(containerId)}/logs/stream?tail=100`;
    
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      setStatus('connected');
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };

    ws.onmessage = (event) => {
      if (isPausedRef.current) return;
      
      try {
        const entry = JSON.parse(event.data) as LogEntry;
        setLogs((prev) => {
          // Keep last 1000 logs to prevent memory issues
          const newLogs = [...prev, entry];
          if (newLogs.length > 1000) return newLogs.slice(newLogs.length - 1000);
          return newLogs;
        });
      } catch (err) {
        console.error('Failed to parse log entry:', err);
      }
    };

    ws.onclose = () => {
      setStatus('disconnected');
      // Attempt to reconnect after 3 seconds
      reconnectTimeoutRef.current = window.setTimeout(connect, 3000);
    };

    ws.onerror = () => {
      setStatus('error');
    };
  }, [host, containerId]);

  useEffect(() => {
    connect();
    return () => {
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current);
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [connect]);

  // Handle auto-scroll
  useEffect(() => {
    if (autoScroll && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  });

  // Detect manual scroll to disable auto-scroll
  const handleScroll = () => {
    if (!scrollContainerRef.current) return;
    
    const { scrollTop, scrollHeight, clientHeight } = scrollContainerRef.current;
    const isAtBottom = Math.abs(scrollHeight - clientHeight - scrollTop) < 10;
    
    if (!isAtBottom && autoScroll) {
      setAutoScroll(false);
    } else if (isAtBottom && !autoScroll) {
      setAutoScroll(true);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm">
      <div className="flex h-full max-h-[80vh] w-full max-w-5xl flex-col overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950 shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-zinc-800 bg-zinc-900/50 px-4 py-3">
          <div className="flex items-center gap-3">
            <h3 className="font-medium text-zinc-100">Logs: {containerName}</h3>
            <div className="flex items-center gap-1.5 text-xs">
              <span className={`relative flex h-2 w-2 rounded-full ${
                status === 'connected' ? 'bg-emerald-500' :
                status === 'connecting' ? 'bg-amber-500' : 'bg-red-500'
              }`}>
                {status === 'connected' && (
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75"></span>
                )}
              </span>
              <span className="text-zinc-400 capitalize">{status}</span>
            </div>
          </div>
          
          <div className="flex items-center gap-2">
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
                isPaused ? 'bg-amber-900/50 text-amber-500 hover:bg-amber-900/70' : 'bg-zinc-800 text-zinc-200 hover:bg-zinc-700'
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

        {/* Log Content */}
        <div 
          ref={scrollContainerRef}
          onScroll={handleScroll}
          className="flex-1 overflow-y-auto p-4 font-mono text-xs text-zinc-300"
        >
          {logs.length === 0 ? (
            <div className="flex h-full items-center justify-center text-zinc-500">
              {status === 'connecting' ? 'Connecting to log stream...' : 'No logs available.'}
            </div>
          ) : (
            <div className="space-y-1">
              {logs.map((log, index) => {
                const uniqueKey = `${log.timestamp}-${index}`;
                return (
                  <div key={uniqueKey} className="flex gap-4 hover:bg-zinc-900/50 px-1 rounded">
                    <span className="shrink-0 text-zinc-600 select-none">
                      {new Date(log.timestamp).toISOString().replace('T', ' ').substring(0, 23)}
                    </span>
                    <span className="break-all whitespace-pre-wrap">{log.message}</span>
                  </div>
                );
              })}
              <div ref={logsEndRef} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
