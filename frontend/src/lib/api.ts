import type { 
  APIResponse, 
  CardDTO, 
  CollectionDTO, 
  CardSearchParams, 
  DashboardStats,
  BulkOperation 
} from "./types";

class APIError extends Error {
  constructor(message: string, public status: number) {
    super(message);
    this.name = "APIError";
  }
}

class APIClient {
  private baseURL: string;

  constructor() {
    // Use Next.js API routes for frontend requests
    this.baseURL = typeof window !== 'undefined' ? '' : (process.env.GO_BACKEND_URL || "http://localhost:8080");
  }

  private async request<T>(url: string, options: RequestInit = {}): Promise<T> {
    try {
      const response = await fetch(url, {
        ...options,
        headers: {
          "Content-Type": "application/json",
          ...options.headers,
        },
        credentials: "include", // Include cookies
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({ 
          message: `HTTP ${response.status}: ${response.statusText}`,
          details: `Failed to fetch from ${url}`
        }));
        
        console.error('API request failed:', {
          url,
          status: response.status,
          statusText: response.statusText,
          error: error.message
        });
        
        throw new APIError(error.message || `Request failed with status ${response.status}`, response.status);
      }

      const result: APIResponse<T> = await response.json();
      
      if (!result.success) {
        console.error('API response indicates failure:', {
          url,
          error: result.error,
          data: result.data
        });
        throw new APIError(result.error || "API request failed", 400);
      }

      return result.data as T;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      
      console.error('Network or parsing error:', {
        url,
        error: error instanceof Error ? error.message : 'Unknown error'
      });
      
      throw new APIError(
        error instanceof Error ? error.message : "Network error occurred", 
        0
      );
    }
  }

  // Dashboard APIs
  async getDashboardStats(): Promise<DashboardStats> {
    return this.request<DashboardStats>(`${this.baseURL}/admin/api/dashboard/stats`);
  }

  // Card APIs
  async searchCards(params: CardSearchParams): Promise<{
    cards: CardDTO[];
    total: number;
    page: number;
    limit: number;
    total_pages: number;
    has_more: boolean;
    has_prev: boolean;
  }> {
    const searchParams = new URLSearchParams();
    
    if (params.search) searchParams.append("search", params.search);
    if (params.collection) searchParams.append("collection", params.collection);
    if (params.level) searchParams.append("level", params.level.toString());
    if (params.animated !== undefined) searchParams.append("animated", params.animated.toString());
    if (params.page) searchParams.append("page", params.page.toString());
    if (params.limit) searchParams.append("limit", params.limit.toString());

    // Use Next.js API route for client-side requests
    const apiUrl = typeof window !== 'undefined' 
      ? `/api/cards?${searchParams}`
      : `${this.baseURL}/admin/api/cards?${searchParams}`;
    
    return this.request(apiUrl);
  }

  async getCard(id: number): Promise<CardDTO> {
    const apiUrl = typeof window !== 'undefined' 
      ? `/api/cards/${id}`
      : `${this.baseURL}/admin/cards/${id}`;
    return this.request<CardDTO>(apiUrl);
  }

  async createCard(card: FormData): Promise<CardDTO> {
    const apiUrl = typeof window !== 'undefined' 
      ? `/api/cards`
      : `${this.baseURL}/admin/cards`;
    
    const response = await fetch(apiUrl, {
      method: "POST",
      body: card,
      credentials: "include",
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: "Unknown error" }));
      throw new APIError(error.message, response.status);
    }

    const result: APIResponse<CardDTO> = await response.json();
    return result.data as CardDTO;
  }

  async updateCard(id: number, card: FormData): Promise<CardDTO> {
    const response = await fetch(`${this.baseURL}/admin/cards/${id}`, {
      method: "PUT",
      body: card,
      credentials: "include",
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: "Unknown error" }));
      throw new APIError(error.message, response.status);
    }

    const result: APIResponse<CardDTO> = await response.json();
    return result.data as CardDTO;
  }

  async deleteCard(id: number): Promise<void> {
    const apiUrl = typeof window !== 'undefined' 
      ? `/api/cards/${id}`
      : `${this.baseURL}/admin/cards/${id}`;
    
    await this.request(apiUrl, {
      method: "DELETE",
    });
  }

  async bulkOperation(operation: BulkOperation): Promise<void> {
    const apiUrl = typeof window !== 'undefined' 
      ? `/api/cards/bulk`
      : `${this.baseURL}/admin/cards/bulk`;
    
    await this.request(apiUrl, {
      method: "POST",
      body: JSON.stringify(operation),
    });
  }

  // Collection APIs
  async getCollections(): Promise<CollectionDTO[]> {
    const apiUrl = typeof window !== 'undefined' 
      ? `/api/collections`
      : `${this.baseURL}/admin/api/collections`;
    return this.request<CollectionDTO[]>(apiUrl);
  }

  async getCollection(id: string): Promise<CollectionDTO> {
    return this.request<CollectionDTO>(`${this.baseURL}/admin/collections/${id}`);
  }

  async getCollectionCards(id: string): Promise<CardDTO[]> {
    const apiUrl = typeof window !== 'undefined' 
      ? `/api/collections/${id}/cards`
      : `${this.baseURL}/admin/api/collections/${id}/cards`;
    return this.request<CardDTO[]>(apiUrl);
  }

  async createCollection(collection: {
    name: string;
    description?: string;
    collection_type: string;
    promo?: boolean;
  }): Promise<CollectionDTO> {
    return this.request<CollectionDTO>(`${this.baseURL}/admin/collections`, {
      method: "POST",
      body: JSON.stringify(collection),
    });
  }

  async updateCollection(id: string, collection: Partial<{
    name: string;
    description: string;
    collection_type: string;
    promo: boolean;
  }>): Promise<CollectionDTO> {
    return this.request<CollectionDTO>(`${this.baseURL}/admin/collections/${id}`, {
      method: "PUT",
      body: JSON.stringify(collection),
    });
  }

  async deleteCollection(id: string): Promise<void> {
    await this.request(`${this.baseURL}/admin/collections/${id}`, {
      method: "DELETE",
    });
  }

  // File upload APIs
  async uploadFiles(files: FileList, onProgress?: (progress: number) => void): Promise<{
    files: Array<{
      filename: string;
      success: boolean;
      size?: number;
      type?: string;
      url?: string;
      error?: string;
    }>;
    total: number;
  }> {
    return new Promise((resolve, reject) => {
      const formData = new FormData();
      
      for (const file of files) {
        formData.append("images", file);
      }

      const xhr = new XMLHttpRequest();

      xhr.upload.addEventListener("progress", (e) => {
        if (e.lengthComputable && onProgress) {
          onProgress((e.loaded / e.total) * 100);
        }
      });

      xhr.onload = () => {
        if (xhr.status === 200) {
          try {
            const result: APIResponse = JSON.parse(xhr.responseText);
            resolve(result.data);
          } catch {
            reject(new APIError("Invalid response format", xhr.status));
          }
        } else {
          try {
            const error = JSON.parse(xhr.responseText);
            reject(new APIError(error.message, xhr.status));
          } catch {
            reject(new APIError("Upload failed", xhr.status));
          }
        }
      };

      xhr.onerror = () => {
        reject(new APIError("Network error", 0));
      };

      xhr.open("POST", `${this.baseURL}/admin/api/upload`);
      xhr.withCredentials = true;
      xhr.send(formData);
    });
  }

  // Auth APIs
  async logout(): Promise<void> {
    await this.request(`${this.baseURL}/auth/logout`, {
      method: "POST",
    });
  }
}

export const apiClient = new APIClient();
export { APIError };