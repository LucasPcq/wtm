// Astro's BASE_URL keeps the configured spelling ('/wtm' or '/'); links append to it.
export const base = import.meta.env.BASE_URL.replace(/\/?$/, '/');
