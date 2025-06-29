// User session types matching Go backend
export interface UserSession {
  discord_id: string;
  username: string;
  avatar: string;
  email: string;
  roles: string[];
  permissions: string[];
  expires_at: string;
  is_admin: boolean;
}

// API response wrapper
export interface APIResponse<T = any> {
  success: boolean;
  message: string;
  data?: T;
  error?: string;
  details?: Record<string, any>;
}

// Card types
export interface Card {
  id: number;
  name: string;
  col_id: string;
  level: number;
  animated: boolean;
  promo: boolean;
  tags: string[];
  created_at: string;
  updated_at: string;
  image_url?: string;
}

export interface CardDTO {
  id: number;
  name: string;
  collection_id: string;
  collection_name: string;
  level: number;
  animated: boolean;
  promo: boolean;
  tags: string[];
  image_url: string;
  created_at: string;
  updated_at: string;
}

// Collection types
export interface Collection {
  id: string;
  name: string;
  description?: string;
  collection_type: "girl_group" | "boy_group" | "other";
  origin: string;
  aliases: string[];
  promo: boolean;
  compressed: boolean;
  fragments: boolean;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface CollectionDTO {
  id: string;
  name: string;
  description?: string;
  collection_type: "girl_group" | "boy_group" | "other";
  origin: string;
  aliases: string[];
  promo: boolean;
  compressed: boolean;
  fragments: boolean;
  tags: string[];
  card_count: number;
  created_at: string;
  updated_at: string;
}

// Search and pagination
export interface CardSearchParams {
  search?: string;
  collection?: string;
  level?: number;
  animated?: boolean;
  page?: number;
  limit?: number;
}

export interface PaginationInfo {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
  has_more: boolean;
  has_prev: boolean;
}

// Dashboard stats
export interface DashboardStats {
  total_cards: number;
  total_collections: number;
  total_users: number;
  sync_percentage: number;
  issue_count: number;
  recent_activity: ActivityItem[];
}

export interface ActivityItem {
  type: string;
  description: string;
  timestamp: string;
}

// Bulk operations
export interface BulkOperation {
  operation: "delete" | "move" | "update";
  card_ids: number[];
  target_collection?: string;
  updates?: Partial<Card>;
}

// File upload
export interface FileUpload {
  name: string;
  size: number;
  type: string;
  data: ArrayBuffer;
}

// Form data types
export interface CardCreateRequest {
  name: string;
  collection_id: string;
  level: number;
  animated: boolean;
  promo: boolean;
  tags: string[];
  image_data?: ArrayBuffer;
  image_name?: string;
}

export interface CardUpdateRequest extends Partial<CardCreateRequest> {
  id: number;
}

// User Management Types
export interface UserDTO {
  id: string;
  discord_id: string;
  username: string;
  discriminator?: string;
  avatar_url?: string;
  is_admin: boolean;
  is_banned: boolean;
  card_count: number;
  collection_count: number;
  last_claim?: string;
  last_daily?: string;
  vials: number;
  gems: number;
  created_at: string;
  updated_at: string;
}

export interface UserSearchParams {
  search?: string;
  is_admin?: boolean;
  is_banned?: boolean;
  page?: number;
  limit?: number;
  sort_by?: 'username' | 'created_at' | 'card_count' | 'last_claim';
  sort_order?: 'asc' | 'desc';
}

export interface UserStats {
  total_users: number;
  active_users: number;
  admin_users: number;
  banned_users: number;
  total_cards_owned: number;
  average_cards_per_user: number;
}

// Sync Management Types
export interface SyncStatus {
  database_healthy: boolean;
  storage_healthy: boolean;
  orphaned_files: number;
  missing_files: number;
  last_sync: string;
  sync_in_progress: boolean;
  consistency_percentage: number;
}

export interface SyncOperation {
  id: string;
  type: 'full_sync' | 'clean_orphans' | 'find_missing' | 'validate_all';
  status: 'pending' | 'running' | 'completed' | 'failed';
  progress: number;
  started_at: string;
  completed_at?: string;
  errors: string[];
  results?: {
    processed: number;
    fixed: number;
    failed: number;
  };
}