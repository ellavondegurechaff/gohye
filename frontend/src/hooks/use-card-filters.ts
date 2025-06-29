import { useState, useCallback } from "react";
import { CardFilters } from "@/types/cards";
import { FILTER_OPTIONS } from "@/constants/cards";

export function useCardFilters(initialFilters: Partial<CardFilters> = {}) {
  const [filters, setFilters] = useState<CardFilters>({
    search: initialFilters.search || "",
    collection: initialFilters.collection || "",
    level: initialFilters.level || "",
    type: initialFilters.type || "",
  });

  const updateFilter = useCallback((key: keyof CardFilters, value: string) => {
    const normalizedValue = value === FILTER_OPTIONS.ALL_COLLECTIONS || 
                           value === FILTER_OPTIONS.ALL_LEVELS || 
                           value === FILTER_OPTIONS.ALL_TYPES ? "" : value;
    
    setFilters(prev => ({
      ...prev,
      [key]: normalizedValue
    }));
  }, []);

  const clearFilters = useCallback(() => {
    setFilters({
      search: "",
      collection: "",
      level: "",
      type: "",
    });
  }, []);

  const hasActiveFilters = useCallback(() => {
    return Object.values(filters).some(value => value !== "");
  }, [filters]);

  return {
    filters,
    updateFilter,
    clearFilters,
    hasActiveFilters: hasActiveFilters(),
  };
}