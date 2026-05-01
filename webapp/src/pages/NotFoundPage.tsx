import { Link } from 'react-router-dom';

export function NotFoundPage() {
  return (
    <div className="flex min-h-[60vh] items-center justify-center text-center">
      <div className="w-full max-w-lg rounded-2xl border border-zinc-800 bg-zinc-900 p-8 shadow-2xl shadow-black/30">
        <p className="text-7xl font-bold tracking-tight text-zinc-100">404</p>
        <h1 className="mt-4 text-2xl font-bold text-zinc-100">Page Not Found</h1>
        <p className="mt-2 text-sm text-zinc-400">The page you're looking for doesn't exist.</p>
        <Link
          to="/"
          className="mt-6 inline-flex rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700"
        >
          Go Home
        </Link>
      </div>
    </div>
  );
}
