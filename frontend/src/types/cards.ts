// Clean separation of card-related types
export interface CardFilters {
  search: string;
  collection: string;
  level: string;
  type: string;
}

export interface CardListState {
  cards: import("@/lib/types").CardDTO[];
  selectedCards: string[];
  loading: boolean;
  error: string | null;
}

export interface CardPaginationState {
  total: number;
  page: number;
  limit: number;
  total_pages: number;
  has_more: boolean;
  has_prev: boolean;
}

// Types are now in constants/cards.ts to reduce duplication