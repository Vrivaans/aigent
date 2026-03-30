/**
 * Base URL del API (incluye prefijo /api).
 * - Producción (Docker / mismo host que el SPA): '/api' resuelve al mismo origen.
 * - Desarrollo: `ng serve` usa proxy.conf.json para enviar /api → http://localhost:3000
 * Si el backend está en otro dominio, cambiá este valor antes del build (p. ej. 'https://api.midominio.com/api').
 */
export const environment = {
  production: false,
  apiBaseUrl: '/api',
};
