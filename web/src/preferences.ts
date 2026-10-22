const FRIENDLY_NAME_KEY = "bigbrother.name";

export function loadFriendlyName(): string {
  try {
    return localStorage.getItem(FRIENDLY_NAME_KEY)?.trim() ?? "";
  } catch {
    return "";
  }
}

export function saveFriendlyName(name: string): string {
  const normalized = name.trim();
  try {
    if (normalized) localStorage.setItem(FRIENDLY_NAME_KEY, normalized);
    else localStorage.removeItem(FRIENDLY_NAME_KEY);
  } catch {
    // The room remains usable if browser storage is disabled.
  }
  return normalized;
}
