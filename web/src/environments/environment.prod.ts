/**
 * Build de producción (`ng build --configuration production`).
 * Por defecto mismo origen que el frontend. Sobrescribí apiBaseUrl si el API vive en otro host.
 */
export const environment = {
  production: true,
  apiBaseUrl: '/api',
};
