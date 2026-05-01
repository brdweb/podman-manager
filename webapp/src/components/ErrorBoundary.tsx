import { Component, type ErrorInfo, type ReactNode } from 'react';

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
  isDetailsOpen: boolean;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = {
    error: null,
    isDetailsOpen: false,
  };

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return { error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('Unhandled render error', error, errorInfo);
  }

  render() {
    const { error, isDetailsOpen } = this.state;

    if (!error) {
      return this.props.children;
    }

    return (
      <div className="flex min-h-screen items-center justify-center bg-zinc-950 px-6 py-12 text-zinc-100">
        <div className="w-full max-w-2xl rounded-2xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl shadow-black/30">
          <p className="text-xs uppercase tracking-[0.28em] text-zinc-500">Podman Manager</p>
          <h1 className="mt-3 text-2xl font-bold text-zinc-100">Something went wrong</h1>
          <p className="mt-2 text-sm text-zinc-400">
            The app ran into an unexpected rendering error. Reloading the page usually restores the
            current session.
          </p>

          <div className="mt-6 rounded-xl border border-red-500/30 bg-red-500/10 p-4">
            <p className="text-sm font-medium text-red-300">{error.message}</p>
          </div>

          {error.stack && (
            <div className="mt-4">
              <button
                type="button"
                onClick={() => this.setState({ isDetailsOpen: !isDetailsOpen })}
                className="rounded-md px-3 py-1.5 text-xs font-medium text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-200"
              >
                {isDetailsOpen ? 'Hide stack trace' : 'Show stack trace'}
              </button>

              {isDetailsOpen && (
                <pre className="mt-3 max-h-64 overflow-auto rounded-xl border border-zinc-800 bg-zinc-950 p-4 text-xs text-zinc-400">
                  {error.stack}
                </pre>
              )}
            </div>
          )}

          <button
            type="button"
            onClick={() => window.location.reload()}
            className="mt-6 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700"
          >
            Reload Page
          </button>
        </div>
      </div>
    );
  }
}
