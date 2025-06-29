import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import { toast } from 'sonner';
import { z } from 'zod';
import { apiClient } from '@/lib/api';
import { CollectionDTO } from '@/lib/types';
import { FileWithPreview } from '@/components/ui/file-upload';

// Validation schemas
const collectionInfoSchema = z.object({
  name: z.string().min(1, "Collection name is required").max(100),
  description: z.string().optional(),
  collection_type: z.enum(["girl_group", "boy_group", "other"]),
});

const cardSchema = z.object({
  name: z.string().min(1, "Card name is required").max(100),
  level: z.number().int().min(1).max(5),
  animated: z.boolean(),
  tags: z.array(z.string()).default([]),
  file: z.instanceof(File),
  preview: z.string().optional(),
});

export type CollectionInfo = z.infer<typeof collectionInfoSchema>;
export type CardInfo = z.infer<typeof cardSchema>;

interface ImportState {
  // Wizard navigation
  currentStep: number;
  
  // Step 1: Collection selection
  selectedCollectionId: string;
  collectionInfo: CollectionInfo | null;
  availableCollections: CollectionDTO[];
  
  // Step 2: File upload
  uploadedFiles: FileWithPreview[];
  
  // Step 3: Card mapping
  parsedCards: CardInfo[];
  
  // Import progress
  isImporting: boolean;
  importProgress: number;
  importErrors: string[];
  
  // Actions
  setCurrentStep: (step: number) => void;
  nextStep: () => void;
  previousStep: () => void;
  
  setSelectedCollectionId: (id: string) => void;
  setCollectionInfo: (info: CollectionInfo | null) => void;
  setAvailableCollections: (collections: CollectionDTO[]) => void;
  
  setUploadedFiles: (files: FileWithPreview[]) => void;
  
  setParsedCards: (cards: CardInfo[]) => void;
  updateCard: (index: number, card: CardInfo) => void;
  
  startImport: () => Promise<void>;
  setImportProgress: (progress: number) => void;
  addImportError: (error: string) => void;
  
  reset: () => void;
}

const initialState = {
  currentStep: 1,
  selectedCollectionId: "",
  collectionInfo: null,
  availableCollections: [],
  uploadedFiles: [],
  parsedCards: [],
  isImporting: false,
  importProgress: 0,
  importErrors: [],
};

export const useImportStore = create<ImportState>()(
  subscribeWithSelector((set, get) => ({
    ...initialState,
    
    setCurrentStep: (step) => set({ currentStep: step }),
    
    nextStep: () => {
      const { currentStep } = get();
      if (currentStep < 4) {
        set({ currentStep: currentStep + 1 });
      }
    },
    
    previousStep: () => {
      const { currentStep } = get();
      if (currentStep > 1) {
        set({ currentStep: currentStep - 1 });
      }
    },
    
    setSelectedCollectionId: (id) => set({ selectedCollectionId: id }),
    
    setCollectionInfo: (info) => {
      // Validate collection info
      if (info) {
        try {
          collectionInfoSchema.parse(info);
          set({ collectionInfo: info });
        } catch (error) {
          console.error('Invalid collection info:', error);
          toast.error('Invalid collection information');
        }
      } else {
        set({ collectionInfo: null });
      }
    },
    
    setAvailableCollections: (collections) => set({ availableCollections: collections }),
    
    setUploadedFiles: (files) => {
      set({ uploadedFiles: files });
      
      // Auto-parse cards from uploaded files
      const parsedCards = files.map((file): CardInfo => {
        const nameWithoutExt = file.name.replace(/\.[^/.]+$/, "");
        
        // Extract level from filename
        const levelMatch = nameWithoutExt.match(/[_-]?(?:l|level)[\s_-]?(\d)/i);
        const level = levelMatch ? Math.min(Math.max(parseInt(levelMatch[1]), 1), 5) : 1;
        
        // Check for animated indicators
        const animated = /(?:animated|anim|gif)/i.test(nameWithoutExt);
        
        // Clean up the name
        let cleanName = nameWithoutExt
          .replace(/[_-]?(?:l|level)[\s_-]?\d/i, "")
          .replace(/[_-]?(?:animated|anim)/i, "")
          .replace(/[_-]+/g, " ")
          .trim();
        
        // Capitalize words
        cleanName = cleanName.replace(/\b\w/g, l => l.toUpperCase());
        
        return {
          name: cleanName || file.name,
          level,
          animated,
          tags: [],
          file,
          preview: file.preview,
        };
      });
      
      set({ parsedCards });
    },
    
    setParsedCards: (cards) => set({ parsedCards: cards }),
    
    updateCard: (index, card) => {
      const { parsedCards } = get();
      try {
        cardSchema.parse(card);
        const updatedCards = [...parsedCards];
        updatedCards[index] = card;
        set({ parsedCards: updatedCards });
      } catch (error) {
        console.error('Invalid card data:', error);
        toast.error('Invalid card information');
      }
    },
    
    startImport: async () => {
      const { selectedCollectionId, collectionInfo, parsedCards } = get();
      
      if (!selectedCollectionId && !collectionInfo) {
        toast.error("Please select a collection or create a new one");
        return;
      }
      
      if (parsedCards.length === 0) {
        toast.error("No cards to import");
        return;
      }
      
      set({ isImporting: true, importProgress: 0, importErrors: [] });
      
      try {
        let targetCollectionId = selectedCollectionId;
        
        // Create new collection if needed
        if (!targetCollectionId && collectionInfo) {
          set({ importProgress: 10 });
          const newCollection = await apiClient.createCollection({
            name: collectionInfo.name,
            description: collectionInfo.description || "",
            collection_type: collectionInfo.collection_type,
            promo: false,
          });
          targetCollectionId = newCollection.id;
          toast.success(`Created collection: ${newCollection.name}`);
        }
        
        // Import cards
        const totalCards = parsedCards.length;
        let successCount = 0;
        
        for (let i = 0; i < totalCards; i++) {
          const card = parsedCards[i];
          
          try {
            const formData = new FormData();
            formData.append("name", card.name);
            formData.append("collection_id", targetCollectionId);
            formData.append("level", card.level.toString());
            formData.append("animated", card.animated.toString());
            formData.append("promo", "false");
            formData.append("tags", JSON.stringify(card.tags));
            formData.append("image", card.file);
            
            await apiClient.createCard(formData);
            successCount++;
            
            // Update progress
            const progress = 10 + ((i + 1) / totalCards) * 90;
            set({ importProgress: progress });
            
            // Small delay to show progress
            await new Promise(resolve => setTimeout(resolve, 100));
          } catch (error: any) {
            get().addImportError(`Failed to import "${card.name}": ${error.message}`);
          }
        }
        
        set({ importProgress: 100 });
        
        if (successCount === totalCards) {
          toast.success(`Successfully imported ${successCount} cards!`);
        } else {
          toast.warning(`Imported ${successCount} of ${totalCards} cards. Check errors for details.`);
        }
      } catch (error: any) {
        console.error('Import failed:', error);
        toast.error(error.message || 'Import failed. Please try again.');
      } finally {
        set({ isImporting: false });
      }
    },
    
    setImportProgress: (progress) => set({ importProgress: progress }),
    
    addImportError: (error) => {
      const { importErrors } = get();
      set({ importErrors: [...importErrors, error] });
    },
    
    reset: () => set(initialState),
  }))
);

// Selectors
export const useImportSelectors = () => {
  const state = useImportStore();
  
  return {
    ...state,
    
    // Computed values
    canProceedFromStep1: !!(state.selectedCollectionId || state.collectionInfo),
    canProceedFromStep2: state.uploadedFiles.length > 0,
    canProceedFromStep3: state.parsedCards.length > 0,
    canStartImport: !!(state.selectedCollectionId || state.collectionInfo) && state.parsedCards.length > 0,
    
    totalCards: state.parsedCards.length,
    animatedCardsCount: state.parsedCards.filter(c => c.animated).length,
    
    selectedCollectionName: state.selectedCollectionId 
      ? state.availableCollections.find(c => c.id === state.selectedCollectionId)?.name
      : state.collectionInfo?.name,
    
    hasErrors: state.importErrors.length > 0,
  };
};