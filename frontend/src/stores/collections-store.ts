import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api';
import { CollectionDTO } from '@/lib/types';
import { collectionSearchSchema, type CollectionSearchValues } from '@/lib/validations';

export interface CollectionsPagination {
  total: number;
  page: number;
  limit: number;
  total_pages: number;
  has_more: boolean;
  has_prev: boolean;
}

export interface CollectionsState {
  // Data
  collections: CollectionDTO[];
  selectedCollection: CollectionDTO | null;
  
  // Search & Filters
  searchParams: CollectionSearchValues;
  
  // Pagination
  pagination: CollectionsPagination;
  
  // UI State
  loading: boolean;
  error: string | null;
  
  // Actions
  setCollections: (collections: CollectionDTO[]) => void;
  setSelectedCollection: (collection: CollectionDTO | null) => void;
  setSearchParams: (params: Partial<CollectionSearchValues>) => void;
  setPagination: (pagination: CollectionsPagination) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
  
  // API Actions
  fetchCollections: (params?: Partial<CollectionSearchValues>) => Promise<void>;
  refreshCollections: () => Promise<void>;
  fetchCollection: (id: string) => Promise<CollectionDTO | null>;
  
  // Utility Actions
  clearError: () => void;
  resetFilters: () => void;
  
  // Collection helpers
  getCollectionById: (id: string) => CollectionDTO | undefined;
  getCollectionsByType: (type: string) => CollectionDTO[];
}

const initialSearchParams: CollectionSearchValues = {
  page: 1,
  limit: 20,
  sort_by: 'created_at',
  sort_order: 'desc',
};

const initialPagination: CollectionsPagination = {
  total: 0,
  page: 1,
  limit: 20,
  total_pages: 0,
  has_more: false,
  has_prev: false,
};

export const useCollectionsStore = create<CollectionsState>()(
  subscribeWithSelector((set, get) => ({
    // Initial state
    collections: [],
    selectedCollection: null,
    searchParams: initialSearchParams,
    pagination: initialPagination,
    loading: false,
    error: null,

    // Basic setters
    setCollections: (collections) => set({ collections }),
    setSelectedCollection: (selectedCollection) => set({ selectedCollection }),
    setSearchParams: (params) => {
      const currentParams = get().searchParams;
      const newParams = { ...currentParams, ...params };
      
      // Validate search params
      try {
        const validatedParams = collectionSearchSchema.parse(newParams);
        set({ searchParams: validatedParams });
      } catch (error) {
        console.warn('Invalid collection search parameters:', error);
        // Use the new params anyway, but log the validation error
        set({ searchParams: newParams });
      }
    },
    setPagination: (pagination) => set({ pagination }),
    setLoading: (loading) => set({ loading }),
    setError: (error) => set({ error }),

    // API Actions
    fetchCollections: async (params) => {
      const state = get();
      const searchParams = params ? { ...state.searchParams, ...params } : state.searchParams;
      
      set({ loading: true, error: null });
      
      try {
        // Validate params before making API call
        const validatedParams = collectionSearchSchema.parse(searchParams);
        
        // Note: Assuming the API will be enhanced to support search/pagination
        // For now, we'll fetch all collections and handle filtering client-side
        const collections = await apiClient.getCollections();
        
        // Apply client-side filtering and pagination
        let filteredCollections = collections;
        
        // Apply search filter
        if (validatedParams.search) {
          const searchTerm = validatedParams.search.toLowerCase();
          filteredCollections = filteredCollections.filter(collection =>
            collection.name.toLowerCase().includes(searchTerm) ||
            collection.description?.toLowerCase().includes(searchTerm)
          );
        }
        
        // Apply collection type filter
        if (validatedParams.collection_type) {
          filteredCollections = filteredCollections.filter(collection =>
            collection.collection_type === validatedParams.collection_type
          );
        }
        
        // Apply promo filter
        if (validatedParams.promo !== undefined) {
          filteredCollections = filteredCollections.filter(collection =>
            collection.promo === validatedParams.promo
          );
        }
        
        // Apply sorting
        filteredCollections.sort((a, b) => {
          const sortBy = validatedParams.sort_by;
          const order = validatedParams.sort_order === 'asc' ? 1 : -1;
          
          switch (sortBy) {
            case 'name':
              return order * a.name.localeCompare(b.name);
            case 'card_count':
              return order * ((a.card_count || 0) - (b.card_count || 0));
            case 'created_at':
              return order * (new Date(a.created_at || 0).getTime() - new Date(b.created_at || 0).getTime());
            case 'updated_at':
              return order * (new Date(a.updated_at || 0).getTime() - new Date(b.updated_at || 0).getTime());
            default:
              return 0;
          }
        });
        
        // Apply pagination
        const total = filteredCollections.length;
        const totalPages = Math.ceil(total / validatedParams.limit);
        const startIndex = (validatedParams.page - 1) * validatedParams.limit;
        const endIndex = startIndex + validatedParams.limit;
        const paginatedCollections = filteredCollections.slice(startIndex, endIndex);
        
        set({
          collections: paginatedCollections,
          pagination: {
            total,
            page: validatedParams.page,
            limit: validatedParams.limit,
            total_pages: totalPages,
            has_more: validatedParams.page < totalPages,
            has_prev: validatedParams.page > 1,
          },
          searchParams: validatedParams,
          loading: false,
        });
      } catch (error: any) {
        console.error('Failed to fetch collections:', error);
        set({ 
          loading: false, 
          error: error.message || 'Failed to load collections' 
        });
        toast.error('Failed to load collections. Please try again.');
      }
    },

    refreshCollections: async () => {
      const { searchParams } = get();
      await get().fetchCollections(searchParams);
    },

    fetchCollection: async (id) => {
      set({ loading: true, error: null });
      
      try {
        const collection = await apiClient.getCollection(id);
        set({ selectedCollection: collection, loading: false });
        return collection;
      } catch (error: any) {
        console.error('Failed to fetch collection:', error);
        set({ 
          loading: false, 
          error: error.message || 'Failed to load collection',
          selectedCollection: null
        });
        toast.error('Failed to load collection details.');
        return null;
      }
    },

    // Utility Actions
    clearError: () => set({ error: null }),

    resetFilters: () => {
      set({ searchParams: initialSearchParams });
      get().fetchCollections(initialSearchParams);
    },

    // Collection helpers
    getCollectionById: (id) => {
      const { collections } = get();
      return collections.find(collection => collection.id === id);
    },

    getCollectionsByType: (type) => {
      const { collections } = get();
      return collections.filter(collection => collection.collection_type === type);
    },
  }))
);

// Selectors for computed values
export const useCollectionsSelectors = () => {
  const state = useCollectionsStore();
  
  return {
    ...state,
    
    // Computed values
    hasCollections: state.collections.length > 0,
    
    // Collection type counts
    girlGroupCount: state.collections.filter(c => c.collection_type === 'girl_group').length,
    boyGroupCount: state.collections.filter(c => c.collection_type === 'boy_group').length,
    soloistCount: 0, // Soloist type not currently supported
    otherCount: state.collections.filter(c => c.collection_type === 'other').length,
    
    // Filter helpers
    hasActiveFilters: Object.keys(state.searchParams).some(
      key => key !== 'page' && key !== 'limit' && key !== 'sort_by' && key !== 'sort_order' && 
             state.searchParams[key as keyof CollectionSearchValues]
    ),
    
    // Total card count across all collections
    totalCardCount: state.collections.reduce((sum, collection) => sum + (collection.card_count || 0), 0),
  };
};

// Subscribe to search params changes for URL sync
export const subscribeToCollectionSearchParams = (callback: (params: CollectionSearchValues) => void) => {
  return useCollectionsStore.subscribe(
    (state) => state.searchParams,
    callback,
    { fireImmediately: false }
  );
};