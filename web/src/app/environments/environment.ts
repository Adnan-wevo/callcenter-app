export const environment = {
  production: false,
  // The Go service. A different origin from the dev server, which is why the
  // backend carries an explicit CORS allow-list (CORS_ORIGINS).
  apiUrl: 'http://localhost:8080',
  version: '0.0.0',
  // 'jssip' is the real engine — SOFTPHONE_PBX_HOST now points at the real
  // staging PBX (sbc.wevetel.com, see docker-compose.yml) and the admin
  // account holds a real assigned extension (5955), so this registers for
  // real over WSS. Set back to 'fake' (FakeSipEngine, no network at all) to
  // develop the call UI without touching the PBX.
  sipEngine: 'jssip' as 'fake' | 'jssip',
};
