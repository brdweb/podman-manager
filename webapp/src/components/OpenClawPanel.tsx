import { useState } from 'react';
import type { FormEvent } from 'react';
import { useOpenClawChat } from '../hooks/useControl';

export function OpenClawPanel() {
  const [message, setMessage] = useState('');
  const [response, setResponse] = useState<string>('');
  const chat = useOpenClawChat();

  async function submit(event: FormEvent) {
    event.preventDefault();
    const trimmed = message.trim();
    if (!trimmed) {
      return;
    }
    const result = await chat.mutateAsync({
      message: trimmed,
      context: { source: 'homelab-control-dashboard' },
    });
    setResponse(renderResponse(result));
    setMessage('');
  }

  return (
    <section className="rounded-lg border border-zinc-800 bg-zinc-900 p-5">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-zinc-100">OpenClaw</h2>
          <p className="mt-1 text-sm text-zinc-500">Chat through the homelab assistant gateway.</p>
        </div>
        <a
          href="https://chat.brdweb.com"
          target="_blank"
          rel="noreferrer"
          className="rounded-md border border-zinc-700 px-3 py-1.5 text-sm text-zinc-300 hover:border-zinc-500"
        >
          Open app
        </a>
      </div>
      <form className="mt-4 space-y-3" onSubmit={submit}>
        <textarea
          value={message}
          onChange={(event) => setMessage(event.target.value)}
          className="field min-h-24 resize-y"
          placeholder="Ask OpenClaw about the current homelab state..."
        />
        <button
          type="submit"
          disabled={chat.isPending}
          className="rounded-md bg-emerald-500 px-4 py-2 text-sm font-medium text-zinc-950 disabled:cursor-not-allowed disabled:opacity-60"
        >
          Send
        </button>
      </form>
      {chat.error && (
        <div className="mt-4 rounded-md border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">
          {chat.error.message}
        </div>
      )}
      {response && (
        <pre className="mt-4 max-h-64 overflow-auto rounded-md bg-zinc-950 p-3 text-sm whitespace-pre-wrap text-zinc-300">
          {response}
        </pre>
      )}
    </section>
  );
}

function renderResponse(value: unknown): string {
  if (typeof value === 'string') {
    return value;
  }
  if (value && typeof value === 'object') {
    const record = value as Record<string, unknown>;
    for (const key of ['message', 'response', 'text', 'content']) {
      if (typeof record[key] === 'string') {
        return record[key] as string;
      }
    }
  }
  return JSON.stringify(value, null, 2);
}
