export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const baseURL = import.meta.env.VITE_API_BASE_URL || "";
  const response = await fetch(`${baseURL}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    },
    ...init
  });

  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }

  return response.json() as Promise<T>;
}

