import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api';
import { CardDTO, CollectionDTO, CardSearchParams } from '@/lib/types';
import { cardSearchSchema, type CardSearchValues } from '@/lib/validations';

export interface CardsPagination {
  total: number;
  page: number;
  limit: number;
  total_pages: number;
  has_more: boolean;
  has_prev: boolean;
}

export interface CardsState {
  // Data
  cards: CardDTO[];
  collections: CollectionDTO[];
  selectedCards: CardDTO[];
  
  // Search & Filters
  searchParams: CardSearchValues;
  
  // Pagination
  pagination: CardsPagination;
  
  // UI State
  loading: boolean;
  error: string | null;
  
  // Bulk Operations
  bulkOperationLoading: boolean;
  
  // Actions
  setCards: (cards: CardDTO[]) => void;
  setCollections: (collections: CollectionDTO[]) => void;
  setSearchParams: (params: Partial<CardSearchValues>) => void;
  setPagination: (pagination: CardsPagination) => void;
  setSelectedCards: (cards: CardDTO[]) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
  
  // API Actions
  fetchCards: (params?: Partial<CardSearchValues>) => Promise<void>;
  refreshCards: () => Promise<void>;
  
  // Card Operations
  deleteCard: (cardId: number) => Promise<boolean>;
  bulkDeleteCards: (cardIds: number[]) => Promise<boolean>;
  bulkMoveCards: (cardIds: number[], targetCollectionId: string) => Promise<boolean>;
  
  // Utility Actions
  clearError: () => void;
  resetFilters: () => void;
  toggleCardSelection: (card: CardDTO) => void;
  selectAllCards: () => void;
  clearSelection: () => void;
}

const initialSearchParams: CardSearchValues = {
  page: 1,
  limit: 50,
  sort_by: 'created_at',
  sort_order: 'desc',
};

const initialPagination: CardsPagination = {
  total: 0,
  page: 1,
  limit: 50,
  total_pages: 0,
  has_more: false,
  has_prev: false,
};

export const useCardsStore = create<CardsState>()(
  subscribeWithSelector((set, get) => ({
    // Initial state
    cards: [],
    collections: [],
    selectedCards: [],
    searchParams: initialSearchParams,
    pagination: initialPagination,
    loading: false,
    error: null,
    bulkOperationLoading: false,

    // Basic setters
    setCards: (cards) => set({ cards }),
    setCollections: (collections) => set({ collections }),
    setSearchParams: (params) => {
      const currentParams = get().searchParams;
      const newParams = { ...currentParams, ...params };
      
      // Validate search params
      try {
        const validatedParams = cardSearchSchema.parse(newParams);
        set({ searchParams: validatedParams });
      } catch (error) {
        console.warn('Invalid search parameters:', error);
        // Use the new params anyway, but log the validation error
        set({ searchParams: newParams });
      }
    },
    setPagination: (pagination) => set({ pagination }),
    setSelectedCards: (selectedCards) => set({ selectedCards }),
    setLoading: (loading) => set({ loading }),
    setError: (error) => set({ error }),

    // API Actions
    fetchCards: async (params) => {
      const state = get();
      const searchParams = params ? { ...state.searchParams, ...params } : state.searchParams;
      
      set({ loading: true, error: null });
      
      try {
        // Validate params before making API call
        const validatedParams = cardSearchSchema.parse(searchParams);
        
        const result = await apiClient.searchCards(validatedParams);
        
        set({
          cards: result.cards,
          pagination: {
            total: result.total,
            page: result.page,
            limit: result.limit,
            total_pages: result.total_pages,
            has_more: result.has_more,
            has_prev: result.has_prev,
          },
          searchParams: validatedParams,
          loading: false,
          selectedCards: [], // Clear selection on new fetch
        });
      } catch (error: any) {
        console.error('Failed to fetch cards:', error);
        set({ 
          loading: false, 
          error: error.message || 'Failed to load cards' 
        });
        toast.error('Failed to load cards. Please try again.');
      }
    },

    refreshCards: async () => {
      const { searchParams } = get();
      await get().fetchCards(searchParams);
    },

    // Card Operations
    deleteCard: async (cardId) => {
      const state = get();
      const card = state.cards.find(c => c.id === cardId);
      
      if (!card) {
        toast.error('Card not found');
        return false;
      }

      if (!confirm(`Are you sure you want to delete "${card.name}"?`)) {
        return false;
      }

      try {
        await apiClient.deleteCard(cardId);
        toast.success('Card deleted successfully');
        
        // Remove card from local state and refresh if needed
        const updatedCards = state.cards.filter(c => c.id !== cardId);
        set({ 
          cards: updatedCards,
          selectedCards: state.selectedCards.filter(c => c.id !== cardId)
        });
        
        // If this was the last card on the page, go to previous page
        if (updatedCards.length === 0 && state.pagination.page > 1) {
          await get().fetchCards({ page: state.pagination.page - 1 });
        }
        
        return true;
      } catch (error: any) {
        console.error('Failed to delete card:', error);
        toast.error(error.message || 'Failed to delete card');
        return false;
      }
    },

    bulkDeleteCards: async (cardIds) => {
      const state = get();
      
      if (cardIds.length === 0) {
        toast.error('No cards selected for deletion');
        return false;
      }

      if (!confirm(`Are you sure you want to delete ${cardIds.length} card(s)?`)) {
        return false;
      }

      set({ bulkOperationLoading: true });

      try {
        await apiClient.bulkOperation({
          operation: 'delete',
          card_ids: cardIds,
        });
        
        toast.success(`${cardIds.length} card(s) deleted successfully`);
        
        // Refresh cards to get updated data
        await get().refreshCards();
        
        set({ bulkOperationLoading: false });
        return true;
      } catch (error: any) {
        console.error('Failed to delete cards:', error);
        toast.error(error.message || 'Failed to delete cards');
        set({ bulkOperationLoading: false });
        return false;
      }
    },

    bulkMoveCards: async (cardIds, targetCollectionId) => {
      const state = get();
      
      if (cardIds.length === 0) {
        toast.error('No cards selected for moving');
        return false;
      }

      if (!targetCollectionId) {
        toast.error('Please select a target collection');
        return false;
      }

      const targetCollection = state.collections.find(c => c.id === targetCollectionId);
      if (!targetCollection) {
        toast.error('Target collection not found');
        return false;
      }

      set({ bulkOperationLoading: true });

      try {
        await apiClient.bulkOperation({
          operation: 'move',
          card_ids: cardIds,
          target_collection: targetCollectionId,
        });
        
        toast.success(`${cardIds.length} card(s) moved to ${targetCollection.name}`);
        
        // Refresh cards to get updated data
        await get().refreshCards();
        
        set({ bulkOperationLoading: false });
        return true;
      } catch (error: any) {
        console.error('Failed to move cards:', error);
        toast.error(error.message || 'Failed to move cards');
        set({ bulkOperationLoading: false });
        return false;
      }
    },

    // Utility Actions
    clearError: () => set({ error: null }),

    resetFilters: () => {
      set({ 
        searchParams: initialSearchParams,
        selectedCards: [],
      });
      get().fetchCards(initialSearchParams);
    },

    toggleCardSelection: (card) => {
      const state = get();
      const isSelected = state.selectedCards.some(c => c.id === card.id);
      
      if (isSelected) {
        set({ 
          selectedCards: state.selectedCards.filter(c => c.id !== card.id) 
        });
      } else {
        set({ 
          selectedCards: [...state.selectedCards, card] 
        });
      }
    },

    selectAllCards: () => {
      const { cards } = get();
      set({ selectedCards: [...cards] });
    },

    clearSelection: () => set({ selectedCards: [] }),
  }))
);

// Selectors for computed values
export const useCardsSelectors = () => {
  const state = useCardsStore();
  
  return {
    ...state,
    
    // Computed values
    hasCards: state.cards.length > 0,
    hasSelection: state.selectedCards.length > 0,
    isAllSelected: state.selectedCards.length === state.cards.length && state.cards.length > 0,
    selectedCardIds: state.selectedCards.map(card => card.id),
    
    // Filter helpers
    hasActiveFilters: Object.keys(state.searchParams).some(
      key => key !== 'page' && key !== 'limit' && key !== 'sort_by' && key !== 'sort_order' && 
             state.searchParams[key as keyof CardSearchValues]
    ),
    
    // Loading states
    isOperationInProgress: state.loading || state.bulkOperationLoading,
  };
};

// Subscribe to search params changes for URL sync
export const subscribeToSearchParams = (callback: (params: CardSearchValues) => void) => {
  return useCardsStore.subscribe(
    (state) => state.searchParams,
    callback,
    { fireImmediately: false }
  );
};