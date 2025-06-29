import { useState, useCallback } from "react";
import type { CardDTO } from "@/lib/types";

export function useCardSelection() {
  const [selectedCards, setSelectedCards] = useState<string[]>([]);

  const toggleCard = useCallback((cardId: string) => {
    setSelectedCards(prev => 
      prev.includes(cardId) 
        ? prev.filter(id => id !== cardId)
        : [...prev, cardId]
    );
  }, []);

  const toggleAll = useCallback((cards: CardDTO[]) => {
    const allCardIds = cards.map(card => card.id.toString());
    setSelectedCards(prev => 
      prev.length === cards.length ? [] : allCardIds
    );
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedCards([]);
  }, []);

  const isSelected = useCallback((cardId: string) => {
    return selectedCards.includes(cardId);
  }, [selectedCards]);

  const isAllSelected = useCallback((cards: CardDTO[]) => {
    return cards.length > 0 && selectedCards.length === cards.length;
  }, [selectedCards]);

  return {
    selectedCards,
    toggleCard,
    toggleAll,
    clearSelection,
    isSelected,
    isAllSelected,
    selectionCount: selectedCards.length,
  };
}