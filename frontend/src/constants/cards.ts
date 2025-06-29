// Card-related constants to reduce magic values
export const CARD_CONFIG = {
  LEVELS: [1, 2, 3, 4, 5] as const,
  TYPES: ["normal", "animated"] as const,
  PAGE_SIZES: [25, 50, 100] as const,
  DEFAULT_PAGE_SIZE: 50,
  DEBOUNCE_DELAY: 500,
} as const;

export const FILTER_OPTIONS = {
  ALL_COLLECTIONS: "all",
  ALL_LEVELS: "all", 
  ALL_TYPES: "all",
} as const;

export const CARD_COLORS = {
  LEVEL_5: "border-yellow-500 text-yellow-400",
  LEVEL_4: "border-purple-500 text-purple-400", 
  LEVEL_3: "border-blue-500 text-blue-400",
  LEVEL_2: "border-green-500 text-green-400",
  LEVEL_1: "border-zinc-500 text-zinc-400",
  ANIMATED: "border-pink-500 text-pink-400",
  PROMO: "border-orange-500 text-orange-400",
} as const;