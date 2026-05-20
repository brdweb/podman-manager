export function resolveIconSrc(icon: string | undefined, url: string): string {
  const value = icon?.trim();
  if (!value) {
    return faviconFor(url);
  }
  if (
    value.startsWith('http://') ||
    value.startsWith('https://') ||
    value.startsWith('data:') ||
    value.startsWith('/')
  ) {
    return value;
  }
  return `https://cdn.jsdelivr.net/gh/selfhst/icons/png/${value}`;
}

export function faviconFor(rawUrl: string): string {
  try {
    const host = new URL(rawUrl).hostname;
    return `https://www.google.com/s2/favicons?domain=${encodeURIComponent(host)}&sz=128`;
  } catch {
    return 'https://cdn.jsdelivr.net/gh/selfhst/icons/png/homepage.png';
  }
}
