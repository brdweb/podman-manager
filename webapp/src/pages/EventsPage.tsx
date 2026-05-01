import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { websocketURL } from '../api/client';
import { useOverview } from '../hooks/useHosts';
import type { PodmanEvent, PodmanEventPayload } from '../types/api';

const MAX_EVENTS = 500;
const EVENT_TYPES = ['container', 'image', 'volume', 'network'] as const;
const RESERVED_DETAIL_KEYS = new Set([
  'Type',
  'type',
  'Action',
  'action',
  'Status',
  'status',
  'ID',
  'id',
  'Name',
  'name',
  'Image',
  'image',
  'Time',
  'time',
  'TimeNano',
  'timeNano',
  'Actor',
  'actor',
  'Attributes',
  'attributes',
]);

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error' | 'paused';
type EventTypeFilter = 'all' | (typeof EVENT_TYPES)[number];

interface LiveEvent {
  sequence: number;
  receivedAt: number;
  host: string;
  type: string;
  action: string;
  entityName: string;
  entityId: string;
  image: string;
  time: string | number | undefined;
  attributes: Record<string, string>;
  raw: PodmanEventPayload;
}

export function EventsPage() {
  const { data: overview } = useOverview();
  const [events, setEvents] = useState<LiveEvent[]>([]);
  const [status, setStatus] = useState<ConnectionStatus>('connecting');
  const [isPaused, setIsPaused] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [typeFilter, setTypeFilter] = useState<EventTypeFilter>('all');
  const [hostFilter, setHostFilter] = useState('all');
  const [error, setError] = useState<string | null>(null);
  const [retryDelay, setRetryDelay] = useState<number | null>(null);

  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const eventsEndRef = useRef<HTMLTableRowElement>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<number | null>(null);
  const reconnectAttemptRef = useRef(0);
  const shouldReconnectRef = useRef(true);
  const sequenceRef = useRef(0);

  const connect = useCallback(() => {
    if (socketRef.current) {
      socketRef.current.close();
    }

    shouldReconnectRef.current = true;
    setStatus('connecting');
    setRetryDelay(null);

    const socket = new WebSocket(websocketURL('/api/events'));
    socketRef.current = socket;

    socket.addEventListener('open', () => {
      reconnectAttemptRef.current = 0;
      setError(null);
      setRetryDelay(null);
      setStatus('connected');
    });

    socket.addEventListener('message', (message) => {
      try {
        const payload = JSON.parse(String(message.data)) as PodmanEvent;
        const normalized = normalizeEvent(payload, ++sequenceRef.current);
        setEvents((current) => [normalized, ...current].slice(0, MAX_EVENTS));
      } catch {
        setError('Received an event payload that could not be parsed.');
      }
    });

    socket.addEventListener('error', () => {
      setError('Event stream connection failed.');
      setStatus('error');
    });

    socket.addEventListener('close', () => {
      if (socketRef.current === socket) {
        socketRef.current = null;
      }

      if (!shouldReconnectRef.current) {
        return;
      }

      const delay = Math.min(30_000, 1_000 * 2 ** reconnectAttemptRef.current);
      reconnectAttemptRef.current += 1;
      setRetryDelay(delay);
      setStatus((current) => (current === 'error' ? 'error' : 'disconnected'));
      reconnectTimerRef.current = window.setTimeout(() => {
        connect();
      }, delay);
    });
  }, []);

  useEffect(() => {
    if (isPaused) {
      shouldReconnectRef.current = false;
      if (reconnectTimerRef.current) {
        window.clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      socketRef.current?.close();
      socketRef.current = null;
      setRetryDelay(null);
      setStatus('paused');
      return;
    }

    connect();

    return () => {
      shouldReconnectRef.current = false;
      if (reconnectTimerRef.current) {
        window.clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [connect, isPaused]);

  const hostOptions = useMemo(() => {
    const hosts = new Set<string>();
    overview?.hosts.forEach((host) => hosts.add(host.name));
    events.forEach((event) => hosts.add(event.host));
    return Array.from(hosts).sort((a, b) => a.localeCompare(b));
  }, [events, overview]);

  const filteredEvents = useMemo(() => {
    return events.filter((event) => {
      const matchesType = typeFilter === 'all' || event.type.toLowerCase() === typeFilter;
      const matchesHost = hostFilter === 'all' || event.host === hostFilter;
      return matchesType && matchesHost;
    });
  }, [events, hostFilter, typeFilter]);

  useEffect(() => {
    if (autoScroll && eventsEndRef.current) {
      eventsEndRef.current.scrollIntoView({ block: 'nearest' });
    }
  }, [autoScroll, filteredEvents.length]);

  const handleScroll = useCallback(() => {
    const container = scrollContainerRef.current;
    if (!container) return;

    const isAtTop = container.scrollTop < 12;
    if (!isAtTop && autoScroll) {
      setAutoScroll(false);
    } else if (isAtTop && !autoScroll) {
      setAutoScroll(true);
    }
  }, [autoScroll]);

  const hasAnyEvents = events.length > 0;

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <div className="mb-3 flex items-center gap-3">
            <h1 className="text-2xl font-bold">Events</h1>
            <ConnectionBadge status={status} />
          </div>
          <p className="text-sm text-zinc-500">
            Live Podman activity across hosts, retained in-memory for the latest {MAX_EVENTS} events.
          </p>
          {retryDelay && status !== 'paused' && (
            <p className="mt-1 text-xs text-zinc-600">
              Reconnecting in {Math.ceil(retryDelay / 1000)} seconds.
            </p>
          )}
        </div>

        <div className="flex flex-col gap-4 md:flex-row md:items-end">
          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-[0.2em] text-zinc-500">
              Type
            </span>
            <select
              value={typeFilter}
              onChange={(event) => setTypeFilter(event.target.value as EventTypeFilter)}
              className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors focus:border-zinc-600 md:w-40"
            >
              <option value="all">All</option>
              {EVENT_TYPES.map((type) => (
                <option key={type} value={type}>
                  {capitalize(type)}
                </option>
              ))}
            </select>
          </label>

          <label className="block">
            <span className="mb-2 block text-xs uppercase tracking-[0.2em] text-zinc-500">
              Host
            </span>
            <select
              value={hostFilter}
              onChange={(event) => setHostFilter(event.target.value)}
              className="w-full rounded-xl border border-zinc-800 bg-zinc-900 px-4 py-2.5 text-sm text-zinc-100 outline-none transition-colors focus:border-zinc-600 md:w-48"
            >
              <option value="all">All hosts</option>
              {hostOptions.map((host) => (
                <option key={host} value={host}>
                  {host}
                </option>
              ))}
            </select>
          </label>

          <button
            type="button"
            onClick={() => setAutoScroll((current) => !current)}
            className={`rounded-xl px-4 py-2.5 text-sm font-medium transition-colors ${
              autoScroll
                ? 'bg-zinc-800 text-zinc-200 hover:bg-zinc-700'
                : 'bg-zinc-900 text-zinc-500 hover:bg-zinc-800 hover:text-zinc-300'
            }`}
          >
            Auto-scroll {autoScroll ? 'On' : 'Off'}
          </button>

          <button
            type="button"
            onClick={() => setIsPaused((current) => !current)}
            className={`rounded-xl px-4 py-2.5 text-sm font-medium transition-colors ${
              isPaused
                ? 'bg-orange-600 text-white hover:bg-orange-500'
                : 'bg-zinc-100 text-zinc-900 hover:bg-white'
            }`}
          >
            {isPaused ? 'Resume' : 'Pause'}
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-6 rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-300">
          {error}
        </div>
      )}

      <div className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-900/50">
        <div
          ref={scrollContainerRef}
          onScroll={handleScroll}
          className="max-h-[calc(100vh-18rem)] overflow-auto"
        >
          <table className="w-full text-left text-sm">
            <thead className="sticky top-0 z-[1] border-b border-zinc-800 bg-zinc-900 text-xs uppercase tracking-wider text-zinc-500">
              <tr>
                <th className="px-4 py-3 font-medium">Time</th>
                <th className="px-4 py-3 font-medium">Host</th>
                <th className="px-4 py-3 font-medium">Type</th>
                <th className="px-4 py-3 font-medium">Action</th>
                <th className="px-4 py-3 font-medium">Entity Name</th>
                <th className="px-4 py-3 font-medium">Details</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-800/50">
              {!hasAnyEvents ? (
                <tr>
                  <td colSpan={6} className="px-4 py-16 text-center">
                    <p className="font-medium text-zinc-300">Waiting for Podman events</p>
                    <p className="mt-1 text-sm text-zinc-500">
                      {status === 'connected'
                        ? 'The stream is connected. New container, image, volume, and network events will appear here.'
                        : 'Connecting to the event stream now.'}
                    </p>
                  </td>
                </tr>
              ) : filteredEvents.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-zinc-500">
                    No events match the current filters.
                  </td>
                </tr>
              ) : (
                filteredEvents.map((event, index) => (
                  <tr
                    key={event.sequence}
                    ref={index === 0 ? eventsEndRef : undefined}
                    className="transition-colors hover:bg-zinc-800/30"
                  >
                    <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-zinc-400">
                      {formatEventTime(event.time, event.receivedAt)}
                    </td>
                    <td className="px-4 py-3 text-zinc-400">
                      <span className="inline-flex items-center rounded-md bg-zinc-800 px-2 py-1 text-xs font-medium text-zinc-300">
                        {event.host}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <TypeBadge type={event.type} />
                    </td>
                    <td className="px-4 py-3 font-medium text-zinc-200">{event.action}</td>
                    <td className="px-4 py-3">
                      <p className="max-w-xs truncate font-medium text-zinc-200">{event.entityName}</p>
                      {event.image && <p className="mt-1 max-w-xs truncate text-xs text-zinc-500">{event.image}</p>}
                    </td>
                    <td className="px-4 py-3 text-xs text-zinc-400">
                      <EventDetails event={event} />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function normalizeEvent(payload: PodmanEvent, sequence: number): LiveEvent {
  const event = payload.event ?? {};
  const actorAttributes = event.Actor?.Attributes ?? event.actor?.attributes ?? {};
  const attributes = event.Attributes ?? event.attributes ?? actorAttributes;
  const entityName = stringValue(event.Name ?? event.name ?? attributes.name ?? attributes.containerName);

  return {
    sequence,
    receivedAt: Date.now(),
    host: payload.host || 'unknown',
    type: stringValue(event.Type ?? event.type, 'unknown').toLowerCase(),
    action: stringValue(event.Action ?? event.action ?? event.Status ?? event.status, 'unknown'),
    entityName: entityName || stringValue(event.ID ?? event.id ?? event.Actor?.ID ?? event.actor?.id, 'unknown'),
    entityId: stringValue(event.ID ?? event.id ?? event.Actor?.ID ?? event.actor?.id),
    image: stringValue(event.Image ?? event.image ?? attributes.image),
    time: event.Time ?? event.time ?? event.TimeNano ?? event.timeNano,
    attributes,
    raw: event,
  };
}

function ConnectionBadge({ status }: { status: ConnectionStatus }) {
  const isConnected = status === 'connected';
  const className = isConnected
    ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300'
    : status === 'paused'
      ? 'border-orange-500/30 bg-orange-500/10 text-orange-300'
      : 'border-red-500/30 bg-red-500/10 text-red-300';

  return (
    <span className={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium ${className}`}>
      <span className={`mr-2 h-1.5 w-1.5 rounded-full ${isConnected ? 'bg-emerald-300' : 'bg-current'}`} />
      {status}
    </span>
  );
}

function TypeBadge({ type }: { type: string }) {
  return (
    <span className="inline-flex rounded-full border border-zinc-700 bg-zinc-800/70 px-2.5 py-0.5 text-xs font-medium text-zinc-300">
      {capitalize(type)}
    </span>
  );
}

function EventDetails({ event }: { event: LiveEvent }) {
  const details = detailEntries(event);

  if (details.length === 0) {
    return <span className="text-zinc-600">-</span>;
  }

  return (
    <div className="max-w-md space-y-1">
      {details.slice(0, 4).map(([key, value]) => (
        <p key={key} className="break-all">
          <span className="font-mono text-zinc-300">{key}</span>
          <span className="mx-2 text-zinc-700">=</span>
          <span className="font-mono text-zinc-500">{value}</span>
        </p>
      ))}
      {details.length > 4 && <p className="text-zinc-600">+{details.length - 4} more</p>}
    </div>
  );
}

function detailEntries(event: LiveEvent): Array<[string, string]> {
  const attributes = Object.entries(event.attributes).map(([key, value]) => [key, String(value)] as [string, string]);
  const raw = Object.entries(event.raw)
    .filter(([key, value]) => !RESERVED_DETAIL_KEYS.has(key) && value !== undefined && value !== null)
    .map(([key, value]) => [key, stringifyDetail(value)] as [string, string]);

  const merged = new Map<string, string>();
  [...attributes, ...raw].forEach(([key, value]) => {
    if (value) merged.set(key, value);
  });

  if (event.entityId) {
    merged.set('id', event.entityId.slice(0, 12));
  }

  return Array.from(merged.entries());
}

function formatEventTime(value: string | number | undefined, fallback: number): string {
  const date = eventDate(value) ?? new Date(fallback);
  return date.toLocaleString();
}

function eventDate(value: string | number | undefined): Date | null {
  if (value === undefined || value === null || value === '') {
    return null;
  }

  if (typeof value === 'number') {
    const milliseconds = value > 10_000_000_000 ? Math.floor(value / 1_000_000) : value * 1000;
    return new Date(milliseconds);
  }

  const parsedNumber = Number(value);
  if (!Number.isNaN(parsedNumber)) {
    return eventDate(parsedNumber);
  }

  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function stringValue(value: unknown, fallback = ''): string {
  if (value === undefined || value === null) {
    return fallback;
  }
  return String(value);
}

function stringifyDetail(value: unknown): string {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  return JSON.stringify(value);
}

function capitalize(value: string): string {
  return value ? `${value.charAt(0).toUpperCase()}${value.slice(1)}` : value;
}
