export const environment = {
  production: false,
  // The Go service. A different origin from the dev server, which is why the
  // backend carries an explicit CORS allow-list (CORS_ORIGINS).
  apiUrl: 'http://localhost:8080',
  version: '0.0.0',
  // 'fake' drives the whole call UI with no PBX (FakeSipEngine) — the default
  // here, since a local dev box usually has no Asterisk to register against.
  // 'jssip' is the real engine. Set to 'jssip' once SOFTPHONE_PBX_HOST etc.
  // point at a real PBX (see docker-compose.yml).
  sipEngine: 'fake' as 'fake' | 'jssip',
};
