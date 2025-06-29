"use client";

import { useEffect, useState, useCallback, useMemo, useRef, memo } from "react";
import { useRouter } from "next/navigation";
import { useDebounce } from "use-debounce";
import { motion, AnimatePresence } from "framer-motion";
import { toast } from "sonner";

// UI Components
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { 
  DropdownMenu, 
  DropdownMenuContent, 
  DropdownMenuItem, 
  DropdownMenuSeparator, 
  DropdownMenuTrigger 
} from "@/components/ui/dropdown-menu";
import { Progress } from "@/components/ui/progress";
import { Slider } from "@/components/ui/slider";

// Icons
import { 
  Plus, Upload, RefreshCw, Search, Filter, Grid3X3, List, 
  MoreVertical, Eye, Edit, Trash2, Download, Heart,
  TrendingUp, Database, Users, Clock, Star, Sparkles,
  Zap, Layers, Palette, Move3D, Maximize2, Minimize2,
  Command, Option, Volume2, VolumeX, Settings,
  BookOpen, Tag, Target, Flame, Crown, Gem, Map,
  RotateCcw, Share2, Copy, ExternalLink, MousePointer
} from "lucide-react";

// Types and Utils
import type { CardSearchParams, CardDTO, CollectionDTO } from "@/lib/types";
import { useCardFilters } from "@/hooks/use-card-filters";
import { useCardSelection } from "@/hooks/use-card-selection";
import { type CardPaginationState } from "@/types/cards";
import { CARD_CONFIG, FILTER_OPTIONS, CARD_COLORS } from "@/constants/cards";
import { getLevelBadgeClasses } from "@/utils/card-helpers";
import { apiClient } from "@/lib/api";

interface RevolutionaryCardsPageProps {
  initialCards: CardDTO[];
  initialCollections: CollectionDTO[];
  initialPagination: CardPaginationState;
  initialSearchParams: CardSearchParams;
}

// Optimized Card Component - Lightweight CSS transitions
const OptimizedCard = memo(({ card, onAction, isSelected, onToggle }: {
  card: CardDTO;
  onAction: (action: string, card: CardDTO) => void;
  isSelected: boolean;
  onToggle: () => void;
}) => {
  const [isHovered, setIsHovered] = useState(false);

  const gradientStyle = useMemo(() => {
    const hue = (card.id * 137.508) % 360; // Golden angle for unique colors
    return {
      background: `linear-gradient(135deg, 
        hsl(${hue}, 70%, 8%) 0%, 
        hsl(${(hue + 60) % 360}, 50%, 12%) 50%, 
        hsl(${(hue + 120) % 360}, 60%, 6%) 100%)`
    };
  }, [card.id]);

  return (
    <div
      className={`group relative cursor-pointer transition-all duration-300 ${
        isHovered ? 'transform -translate-y-2 scale-105' : ''
      }`}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      <div className="relative w-full">
        <Card className="overflow-hidden border-0 bg-transparent shadow-2xl shadow-black/50">
          {/* Dynamic Background */}
          <div 
            className="absolute inset-0 opacity-90"
            style={gradientStyle}
          />
          
          {/* Glassmorphism Overlay */}
          <div className="absolute inset-0 bg-gradient-to-br from-white/10 via-transparent to-black/20 backdrop-blur-[1px]" />
          
          {/* Holographic Border Effect */}
          <div className="absolute inset-0 rounded-lg bg-gradient-to-r from-pink-500/20 via-purple-500/20 to-cyan-500/20 opacity-0 group-hover:opacity-100 transition-opacity duration-500" 
               style={{ padding: '1px' }}>
            <div className="w-full h-full rounded-lg bg-black/90" />
          </div>

          {/* Card Image - Optimized */}
          <div className="relative aspect-[3/4] overflow-hidden">
            {card.image_url ? (
              <div className="w-full h-full">
                <img
                  src={card.image_url}
                  alt={card.name}
                  className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
                />
              </div>
            ) : (
              <div className="w-full h-full bg-gradient-to-br from-zinc-800 to-zinc-700 flex items-center justify-center">
                <Database className="h-16 w-16 text-zinc-500" />
              </div>
            )}
            
            {/* Floating Selection Checkbox */}
            <div className="absolute top-4 left-4 z-20 transition-transform duration-200 hover:scale-110 active:scale-95">
              <div className="p-2 rounded-full bg-black/40 backdrop-blur-md border border-white/20">
                <Checkbox
                  checked={isSelected}
                  onCheckedChange={onToggle}
                  className="bg-transparent border-white/60 data-[state=checked]:bg-pink-500 data-[state=checked]:border-pink-500"
                />
              </div>
            </div>

            {/* Floating Action Menu */}
            <div className="absolute top-4 right-4 z-20 opacity-0 group-hover:opacity-100 transition-all duration-300 hover:scale-105">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button 
                    size="sm" 
                    className="h-10 w-10 p-0 rounded-full bg-black/40 backdrop-blur-md border border-white/20 hover:bg-black/60 transition-all duration-300"
                  >
                    <MoreVertical className="h-4 w-4 text-white" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent 
                  align="end" 
                  className="w-56 bg-black/90 backdrop-blur-xl border border-white/10 text-white"
                >
                  {[
                    { icon: Eye, label: "View Details", action: "view", shortcut: "⌘V" },
                    { icon: Edit, label: "Edit Card", action: "edit", shortcut: "⌘E" },
                    { icon: Copy, label: "Duplicate", action: "duplicate", shortcut: "⌘D" },
                    { icon: Share2, label: "Share", action: "share", shortcut: "⌘S" },
                    { icon: Download, label: "Download", action: "download", shortcut: "⌘↓" },
                    null, // Separator
                    { icon: Trash2, label: "Delete", action: "delete", shortcut: "⌫", destructive: true },
                  ].map((item, index) => 
                    item === null ? (
                      <DropdownMenuSeparator key={index} className="bg-white/10" />
                    ) : (
                      <DropdownMenuItem 
                        key={item.action}
                        onClick={() => onAction(item.action, card)}
                        className={`cursor-pointer group/item ${
                          item.destructive ? 'text-red-400 focus:text-red-300 focus:bg-red-500/10' : 'text-white focus:text-white focus:bg-white/10'
                        }`}
                      >
                        <item.icon className="mr-3 h-4 w-4" />
                        <span className="flex-1">{item.label}</span>
                        <span className="text-xs text-zinc-500 group-focus/item:text-zinc-400">{item.shortcut}</span>
                      </DropdownMenuItem>
                    )
                  )}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>

            {/* Level Badge with Glow Effect */}
            <div className="absolute bottom-4 left-4 z-10 transition-transform duration-200 hover:scale-110">
              <Badge 
                className={`${getLevelBadgeClasses(card.level)} relative overflow-hidden backdrop-blur-md shadow-lg`}
                style={{
                  boxShadow: `0 0 20px ${
                    card.level === 5 ? '#fbbf24' :
                    card.level === 4 ? '#a855f7' :
                    card.level === 3 ? '#3b82f6' :
                    card.level === 2 ? '#10b981' : '#6b7280'
                  }40`
                }}
              >
                <Crown className="mr-1 h-3 w-3" />
                Level {card.level}
                {/* Shimmer effect */}
                <div className="absolute inset-0 -skew-x-12 bg-gradient-to-r from-transparent via-white/20 to-transparent animate-shimmer" />
              </Badge>
            </div>

            {/* Type Badges */}
            <div className="absolute bottom-4 right-4 z-10 flex gap-2">
              {card.animated && (
                <div className="transition-transform duration-200 hover:scale-110">
                  <Badge className="bg-pink-500/20 text-pink-300 border-pink-500/30 backdrop-blur-md shadow-lg shadow-pink-500/20">
                    <Sparkles className="mr-1 h-3 w-3 animate-pulse" />
                    Animated
                  </Badge>
                </div>
              )}
              {card.promo && (
                <div className="transition-transform duration-200 hover:scale-110">
                  <Badge className="bg-orange-500/20 text-orange-300 border-orange-500/30 backdrop-blur-md shadow-lg shadow-orange-500/20">
                    <Flame className="mr-1 h-3 w-3 animate-bounce" />
                    Promo
                  </Badge>
                </div>
              )}
            </div>

            {/* Interaction Glow */}
            <div className="absolute inset-0 bg-gradient-radial from-white/10 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
          </div>

          {/* Card Info - Optimized */}
          <CardContent className="relative p-6 bg-black/50 backdrop-blur-md">
            <div className="space-y-3">
              <h3 className="font-bold text-xl text-white truncate group-hover:text-transparent group-hover:bg-gradient-to-r group-hover:from-pink-400 group-hover:to-purple-400 group-hover:bg-clip-text transition-all duration-300">
                {card.name}
              </h3>
              <p className="text-sm text-zinc-300 truncate opacity-80 group-hover:opacity-100 transition-opacity">
                {card.collection_name}
              </p>
              {card.tags && card.tags.length > 0 && (
                <div className="flex flex-wrap gap-1">
                  {card.tags.slice(0, 3).map((tag, index) => (
                    <div key={index} className="transition-transform duration-200 hover:scale-105">
                      <Badge variant="outline" className="text-xs border-zinc-600/50 text-zinc-400 bg-zinc-800/50 backdrop-blur-sm">
                        #{tag}
                      </Badge>
                    </div>
                  ))}
                  {card.tags.length > 3 && (
                    <Badge variant="outline" className="text-xs border-zinc-600/50 text-zinc-400 bg-zinc-800/50 backdrop-blur-sm">
                      +{card.tags.length - 3}
                    </Badge>
                  )}
                </div>
              )}
              
              {/* Metadata */}
              <div className="flex items-center justify-between text-xs text-zinc-500 pt-2 border-t border-zinc-700/50">
                <span className="flex items-center gap-1">
                  <Clock className="h-3 w-3" />
                  {new Date(card.created_at).toLocaleDateString()}
                </span>
                <span className="flex items-center gap-1">
                  <Target className="h-3 w-3" />
                  ID: {card.id}
                </span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
});

// Smart Search with Real-time Suggestions
const IntelligentSearchBar = ({ 
  value, 
  onChange, 
  suggestions,
  onSuggestionSelect 
}: {
  value: string;
  onChange: (value: string) => void;
  suggestions: string[];
  onSuggestionSelect: (suggestion: string) => void;
}) => {
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showSuggestions || suggestions.length === 0) return;

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex(prev => Math.min(prev + 1, suggestions.length - 1));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex(prev => Math.max(prev - 1, -1));
        break;
      case 'Enter':
        e.preventDefault();
        if (selectedIndex >= 0) {
          onSuggestionSelect(suggestions[selectedIndex]);
          setShowSuggestions(false);
        }
        break;
      case 'Escape':
        setShowSuggestions(false);
        setSelectedIndex(-1);
        break;
    }
  };

  return (
    <div className="relative">
      <div className="relative group">
        <Search className="absolute left-4 top-1/2 transform -translate-y-1/2 h-5 w-5 text-zinc-400 group-focus-within:text-pink-400 transition-colors" />
        <Input
          placeholder="Search by name, collection, tags... (⌘K)"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onFocus={() => setShowSuggestions(true)}
          onBlur={() => setTimeout(() => setShowSuggestions(false), 200)}
          onKeyDown={handleKeyDown}
          className="pl-12 pr-16 h-14 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white placeholder:text-zinc-500 focus:border-pink-500/50 focus:ring-pink-500/20 focus:ring-2 transition-all duration-300 text-lg"
        />
        <div className="absolute right-4 top-1/2 transform -translate-y-1/2 flex items-center gap-2">
          <kbd className="px-2 py-1 text-xs bg-zinc-800/50 border border-zinc-700/50 rounded text-zinc-400">
            ⌘K
          </kbd>
        </div>
      </div>

      {/* Smart Suggestions */}
      <AnimatePresence>
        {showSuggestions && suggestions.length > 0 && (
          <motion.div
            initial={{ opacity: 0, y: -10, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -10, scale: 0.95 }}
            transition={{ duration: 0.15 }}
            className="absolute top-full left-0 right-0 z-50 mt-2 bg-black/90 backdrop-blur-xl border border-zinc-800/50 rounded-lg shadow-2xl overflow-hidden"
          >
            {suggestions.slice(0, 8).map((suggestion, index) => (
              <motion.div
                key={suggestion}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: index * 0.05 }}
                className={`px-4 py-3 cursor-pointer transition-colors ${
                  selectedIndex === index 
                    ? 'bg-pink-500/20 text-pink-300' 
                    : 'text-zinc-300 hover:bg-zinc-800/50'
                }`}
                onMouseDown={() => onSuggestionSelect(suggestion)}
                onMouseEnter={() => setSelectedIndex(index)}
              >
                <div className="flex items-center gap-3">
                  <Search className="h-4 w-4 opacity-50" />
                  <span>{suggestion}</span>
                </div>
              </motion.div>
            ))}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
};

export function RevolutionaryCardsPage({
  initialCards,
  initialCollections,
  initialPagination,
  initialSearchParams,
}: RevolutionaryCardsPageProps) {
  const router = useRouter();
  
  // Enhanced State Management
  const [cards, setCards] = useState<CardDTO[]>(initialCards);
  const [collections] = useState<CollectionDTO[]>(initialCollections);
  const [pagination, setPagination] = useState<CardPaginationState>(initialPagination);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<'grid' | 'list' | 'masonry'>('masonry');
  const [showFilters, setShowFilters] = useState(false);
  const [soundEnabled, setSoundEnabled] = useState(true);
  const [cardSize, setCardSize] = useState(300);
  const [searchSuggestions, setSearchSuggestions] = useState<string[]>([]);

  // Custom Hooks
  const { filters, updateFilter, clearFilters, hasActiveFilters } = useCardFilters({
    search: initialSearchParams.search,
    collection: initialSearchParams.collection,
    level: initialSearchParams.level?.toString(),
    type: initialSearchParams.animated === true ? "animated" : 
          initialSearchParams.animated === false ? "normal" : "",
  });

  const {
    selectedCards,
    toggleCard,
    toggleAll,
    clearSelection,
    isSelected,
    isAllSelected,
    selectionCount,
  } = useCardSelection();

  const [debouncedSearchTerm] = useDebounce(filters.search, CARD_CONFIG.DEBOUNCE_DELAY);

  // Enhanced Computed Values
  const stats = useMemo(() => ({
    total: pagination.total,
    animated: cards.filter(c => c.animated).length,
    promo: cards.filter(c => c.promo).length,
    collections: new Set(cards.map(c => c.collection_id)).size,
    levels: cards.reduce((acc, card) => {
      acc[card.level] = (acc[card.level] || 0) + 1;
      return acc;
    }, {} as Record<number, number>),
    averageLevel: cards.length > 0 ? cards.reduce((sum, card) => sum + card.level, 0) / cards.length : 0,
  }), [cards, pagination.total]);

  // Generate search suggestions
  useEffect(() => {
    if (!debouncedSearchTerm) {
      setSearchSuggestions([]);
      return;
    }

    const suggestions = new Set<string>();
    const term = debouncedSearchTerm.toLowerCase();

    // Add card name suggestions
    cards.forEach(card => {
      if (card.name.toLowerCase().includes(term)) {
        suggestions.add(card.name);
      }
    });

    // Add collection suggestions  
    collections.forEach(collection => {
      if (collection.name.toLowerCase().includes(term)) {
        suggestions.add(collection.name);
      }
    });

    // Add tag suggestions
    cards.forEach(card => {
      card.tags?.forEach(tag => {
        if (tag.toLowerCase().includes(term)) {
          suggestions.add(`#${tag}`);
        }
      });
    });

    setSearchSuggestions(Array.from(suggestions).slice(0, 8));
  }, [debouncedSearchTerm, cards, collections]);

  // Enhanced API Operations
  const fetchCards = useCallback(async () => {
    setLoading(true);
    setError(null);
    
    try {
      const params: CardSearchParams = {
        page: 1,
        limit: CARD_CONFIG.DEFAULT_PAGE_SIZE,
      };
      
      if (debouncedSearchTerm) params.search = debouncedSearchTerm;
      if (filters.collection) params.collection = filters.collection;
      if (filters.level) params.level = parseInt(filters.level);
      if (filters.type === "animated") params.animated = true;
      if (filters.type === "normal") params.animated = false;
      
      const result = await apiClient.searchCards(params);
      setCards(result.cards);
      setPagination({
        total: result.total,
        page: result.page,
        limit: result.limit,
        total_pages: result.total_pages,
        has_more: result.has_more,
        has_prev: result.has_prev,
      });
      
      clearSelection();
      
      // Play search sound
      if (soundEnabled) {
        const audio = new Audio('data:audio/wav;base64,UklGRnoGAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQoGAACBhYqFbF1fdJivrJBhNjVgodDbq2EcBj+a2/LDciUFLIHO8tiJNwgZaLvt559NEAxQp+PwtmMcBjiR1/LMeSwFJHfH8N2QQAoUXrTp66hVFApGn+DyvmEXBze/zPLHdSMELIHO8tuFNwgZZ7zs5ZdMEAxQp+PwtmUcBzaRz+3PgykEJ2+66rBuFgU7hM7y2YU3CBlkvuzjl0wQDFCn4/C2ZRwGNZHP7K+DIggudM7u3IU7CRZq6uW7fBYDMn/P8N2NQAoTXrTp66hVFApPiOLxy2Y/CzODwcVrPAq/hM7y2YU4CRNlu+zhnUsQDFCn4/C2ZRwHNZPO7K+DIgcme82+dRUFNnfM8N+QQAoUWqzn6a5hFgo0Vn/C7r1bGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJtteKdVxYKRoPH6bRmHAc5hM7y2YU3CRNluuzjmE4QDFCn4/C2ZRwHNZLO7K+DIgcme82+dRUFN3zK8N+QQQsUWqzn6a5hFgo0Vn/C7r1bGgwzf8nw34Y/CiaAyPDajTsIG2e86qxCFwRGiuvyv2gaCj2AzPLHdCcEKne66K2DIgctjMrt5ZNIDR5ryPHRhzIJGF615OOiSQwQRa3l8bdhFgo2k8/sz4EqBypWyuqjTgwRSarl8bdhFgo2k8/sz4EpBipwyuqgTAsMSaq68LRjFgo2k8/sz4AkBipwyuqgTAsMSaq68LRjFgo2k8/sz4ApBipwyuqgTAwQD15q9+G2ZRwHNZLP7M+DIgctdM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKe=');
        audio.volume = 0.1;
        audio.play().catch(() => {});
      }
    } catch (error: any) {
      console.error('Failed to fetch cards:', error);
      setError(error.message || 'Failed to load cards');
      toast.error('Failed to load cards');
    } finally {
      setLoading(false);
    }
  }, [debouncedSearchTerm, filters.collection, filters.level, filters.type, clearSelection, soundEnabled]);

  const handleCardAction = useCallback(async (action: string, card: CardDTO) => {
    switch (action) {
      case 'view':
        router.push(`/dashboard/cards/${card.id}`);
        break;
      case 'edit':
        router.push(`/dashboard/cards/${card.id}/edit`);
        break;
      case 'duplicate':
        // Add duplication logic
        toast.success(`Duplicated "${card.name}"`);
        break;
      case 'share':
        if (navigator.share) {
          navigator.share({
            title: card.name,
            text: `Check out this ${card.collection_name} card!`,
            url: window.location.href + `/${card.id}`
          });
        } else {
          navigator.clipboard.writeText(window.location.href + `/${card.id}`);
          toast.success('Link copied to clipboard');
        }
        break;
      case 'download':
        window.open(card.image_url, '_blank');
        break;
      case 'delete':
        if (confirm(`Delete "${card.name}"?`)) {
          try {
            await apiClient.deleteCard(card.id);
            toast.success(`Deleted "${card.name}"`);
            fetchCards();
          } catch (error: any) {
            toast.error('Failed to delete card');
          }
        }
        break;
    }
  }, [router, fetchCards]);

  const handleBulkDelete = useCallback(async () => {
    if (selectionCount === 0) return;
    
    if (!confirm(`Are you sure you want to delete ${selectionCount} card(s)?`)) {
      return;
    }

    try {
      setLoading(true);
      await apiClient.bulkOperation({
        operation: 'delete',
        card_ids: selectedCards.map(id => parseInt(id)),
      });
      toast.success(`Deleted ${selectionCount} card(s)`);
      clearSelection();
      await fetchCards();
    } catch (error: any) {
      console.error('Failed to delete cards:', error);
      toast.error(error.message || 'Failed to delete cards');
    } finally {
      setLoading(false);
    }
  }, [selectedCards, selectionCount, clearSelection, fetchCards]);

  useEffect(() => {
    fetchCards();
  }, [fetchCards]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey) {
        switch (e.key) {
          case 'k':
            e.preventDefault();
            (document.querySelector('input[placeholder*="Search"]') as HTMLInputElement)?.focus();
            break;
          case 'f':
            e.preventDefault();
            setShowFilters(!showFilters);
            break;
          case 'r':
            e.preventDefault();
            fetchCards();
            break;
          case 'n':
            e.preventDefault();
            router.push('/dashboard/cards/new');
            break;
        }
      }
      
      if (e.key === 'Escape') {
        clearSelection();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [showFilters, fetchCards, router, clearSelection]);

  return (
    <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900 to-black relative overflow-hidden">
      {/* Animated Background */}
      <div className="absolute inset-0 opacity-30">
        <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-pink-500/10 rounded-full blur-3xl animate-pulse" />
        <div className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-purple-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '2s' }} />
        <div className="absolute top-1/2 left-1/2 w-96 h-96 bg-cyan-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '4s' }} />
      </div>

      {/* Header Section */}
      <div className="relative z-10 border-b border-white/5 bg-black/40 backdrop-blur-2xl">
        <div className="container mx-auto px-6 py-8">
          {/* Breadcrumb */}
          <motion.div 
            initial={{ opacity: 0, y: -20 }}
            animate={{ opacity: 1, y: 0 }}
            className="flex items-center gap-2 text-sm text-zinc-500 mb-6"
          >
            <span>Dashboard</span>
            <span>/</span>
            <span className="text-white">Cards</span>
          </motion.div>

          {/* Main Header */}
          <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-6 mb-8">
            <motion.div
              initial={{ opacity: 0, x: -30 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: 0.2 }}
            >
              <h1 className="text-5xl font-black bg-gradient-to-r from-white via-pink-200 to-purple-200 bg-clip-text text-transparent">
                Card Collection
              </h1>
              <p className="text-zinc-400 text-xl mt-3 flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-pink-400" />
                Revolutionary K-pop trading card experience
              </p>
            </motion.div>

            <motion.div 
              initial={{ opacity: 0, x: 30 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: 0.4 }}
              className="flex flex-wrap gap-3"
            >
              <Button
                variant="outline"
                onClick={fetchCards}
                disabled={loading}
                className="border-zinc-700/50 hover:bg-zinc-800/50 transition-all duration-300 backdrop-blur-sm"
              >
                <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
                Refresh
                <kbd className="hidden sm:inline-flex ml-2 px-1.5 py-0.5 text-xs bg-zinc-800/50 border border-zinc-700/50 rounded">⌘R</kbd>
              </Button>
              <Button
                variant="outline"
                onClick={() => router.push('/dashboard/import')}
                className="border-zinc-700/50 hover:bg-zinc-800/50 transition-all duration-300 backdrop-blur-sm"
              >
                <Upload className="mr-2 h-4 w-4" />
                Import Album
              </Button>
              <Button
                onClick={() => router.push('/dashboard/cards/new')}
                className="bg-gradient-to-r from-pink-600 via-pink-500 to-purple-600 hover:from-pink-700 hover:via-pink-600 hover:to-purple-700 text-white shadow-2xl shadow-pink-500/25 transition-all duration-300"
              >
                <Plus className="mr-2 h-4 w-4" />
                Add Card
                <kbd className="hidden sm:inline-flex ml-2 px-1.5 py-0.5 text-xs bg-white/20 border border-white/20 rounded">⌘N</kbd>
              </Button>
            </motion.div>
          </div>

          {/* Enhanced Stats Dashboard */}
          <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-4">
            {[
              { icon: Database, label: "Total Cards", value: stats.total, color: "pink", gradient: "from-pink-500 to-rose-500" },
              { icon: Sparkles, label: "Animated", value: stats.animated, color: "purple", gradient: "from-purple-500 to-violet-500" },
              { icon: Flame, label: "Promo Cards", value: stats.promo, color: "orange", gradient: "from-orange-500 to-amber-500" },
              { icon: Users, label: "Collections", value: stats.collections, color: "blue", gradient: "from-blue-500 to-cyan-500" },
              { icon: TrendingUp, label: "Avg Level", value: stats.averageLevel.toFixed(1), color: "green", gradient: "from-green-500 to-emerald-500" },
              { icon: Crown, label: "Level 5", value: stats.levels[5] || 0, color: "yellow", gradient: "from-yellow-500 to-amber-500" },
            ].map((stat, index) => (
              <motion.div
                key={stat.label}
                initial={{ opacity: 0, y: 20, scale: 0.9 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                transition={{ delay: 0.6 + index * 0.1 }}
                whileHover={{ scale: 1.05 }}
                className="group cursor-pointer"
              >
                <Card className="bg-gradient-to-br from-black/60 to-zinc-900/60 backdrop-blur-xl border-white/10 hover:border-white/20 transition-all duration-300 overflow-hidden relative">
                  <div className={`absolute inset-0 bg-gradient-to-br ${stat.gradient} opacity-0 group-hover:opacity-10 transition-opacity duration-300`} />
                  <CardContent className="p-4 relative z-10">
                    <div className="flex items-center gap-3">
                      <div className={`p-3 rounded-xl bg-gradient-to-br ${stat.gradient} shadow-lg`}>
                        <stat.icon className="h-5 w-5 text-white" />
                      </div>
                      <div>
                        <p className="text-2xl font-bold text-white group-hover:text-transparent group-hover:bg-gradient-to-r group-hover:bg-clip-text group-hover:from-white group-hover:to-zinc-300 transition-all duration-300">
                          {typeof stat.value === 'number' ? stat.value.toLocaleString() : stat.value}
                        </p>
                        <p className="text-xs text-zinc-400 uppercase tracking-wider font-medium">{stat.label}</p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </motion.div>
            ))}
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="relative z-10 container mx-auto px-6 py-8 space-y-8">
        {/* Revolutionary Search and Controls */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.8 }}
        >
          <Card className="bg-gradient-to-br from-black/80 via-zinc-900/60 to-black/80 backdrop-blur-2xl border-white/10 shadow-2xl overflow-hidden">
            <CardContent className="p-8">
              <div className="space-y-6">
                {/* Intelligent Search */}
                <IntelligentSearchBar
                  value={filters.search}
                  onChange={(value) => updateFilter("search", value)}
                  suggestions={searchSuggestions}
                  onSuggestionSelect={(suggestion) => updateFilter("search", suggestion)}
                />

                {/* Advanced Controls */}
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div className="flex items-center gap-4">
                    {/* Filter Toggle */}
                    <Button
                      variant="outline"
                      onClick={() => setShowFilters(!showFilters)}
                      className="border-zinc-700/50 hover:bg-zinc-800/50 backdrop-blur-sm transition-all duration-300"
                    >
                      <Filter className="mr-2 h-4 w-4" />
                      Filters
                      <kbd className="hidden sm:inline-flex ml-2 px-1.5 py-0.5 text-xs bg-zinc-800/50 border border-zinc-700/50 rounded">⌘F</kbd>
                      {hasActiveFilters && (
                        <Badge variant="secondary" className="ml-2 bg-pink-500/20 text-pink-300 border-pink-500/30">
                          {Object.values(filters).filter(Boolean).length}
                        </Badge>
                      )}
                    </Button>

                    {/* View Mode Toggle */}
                    <Tabs value={viewMode} onValueChange={(value) => setViewMode(value as any)}>
                      <TabsList className="bg-black/60 backdrop-blur-xl border border-zinc-800/50">
                        <TabsTrigger value="masonry" className="data-[state=active]:bg-zinc-700/50">
                          <Layers className="h-4 w-4" />
                        </TabsTrigger>
                        <TabsTrigger value="grid" className="data-[state=active]:bg-zinc-700/50">
                          <Grid3X3 className="h-4 w-4" />
                        </TabsTrigger>
                        <TabsTrigger value="list" className="data-[state=active]:bg-zinc-700/50">
                          <List className="h-4 w-4" />
                        </TabsTrigger>
                      </TabsList>
                    </Tabs>

                    {/* Card Size Slider */}
                    <div className="hidden lg:flex items-center gap-3 min-w-[200px]">
                      <Minimize2 className="h-4 w-4 text-zinc-400" />
                      <Slider
                        value={[cardSize]}
                        onValueChange={([value]) => setCardSize(value)}
                        max={400}
                        min={200}
                        step={25}
                        className="flex-1"
                      />
                      <Maximize2 className="h-4 w-4 text-zinc-400" />
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    {/* Sound Toggle */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setSoundEnabled(!soundEnabled)}
                      className="text-zinc-400 hover:text-white"
                    >
                      {soundEnabled ? <Volume2 className="h-4 w-4" /> : <VolumeX className="h-4 w-4" />}
                    </Button>

                    <span className="text-sm text-zinc-400">
                      {cards.length} of {stats.total} cards
                    </span>
                  </div>
                </div>

                {/* Expandable Advanced Filters */}
                <AnimatePresence>
                  {showFilters && (
                    <motion.div
                      initial={{ height: 0, opacity: 0 }}
                      animate={{ height: "auto", opacity: 1 }}
                      exit={{ height: 0, opacity: 0 }}
                      className="overflow-hidden"
                    >
                      <Separator className="bg-white/10 my-6" />
                      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-6">
                        {/* Collection Filter */}
                        <div className="space-y-3">
                          <label className="text-sm font-semibold text-zinc-300 flex items-center gap-2">
                            <BookOpen className="h-4 w-4" />
                            Collection
                          </label>
                          <Select 
                            value={filters.collection || FILTER_OPTIONS.ALL_COLLECTIONS} 
                            onValueChange={(value) => updateFilter("collection", value)}
                          >
                            <SelectTrigger className="bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                              <SelectItem value={FILTER_OPTIONS.ALL_COLLECTIONS}>All Collections</SelectItem>
                              {collections.map((collection) => (
                                <SelectItem key={collection.id} value={collection.id}>
                                  {collection.name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>

                        {/* Level Filter */}
                        <div className="space-y-3">
                          <label className="text-sm font-semibold text-zinc-300 flex items-center gap-2">
                            <Crown className="h-4 w-4" />
                            Level
                          </label>
                          <Select 
                            value={filters.level || FILTER_OPTIONS.ALL_LEVELS} 
                            onValueChange={(value) => updateFilter("level", value)}
                          >
                            <SelectTrigger className="bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                              <SelectItem value={FILTER_OPTIONS.ALL_LEVELS}>All Levels</SelectItem>
                              {CARD_CONFIG.LEVELS.map((level) => (
                                <SelectItem key={level} value={level.toString()}>
                                  Level {level}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>

                        {/* Type Filter */}
                        <div className="space-y-3">
                          <label className="text-sm font-semibold text-zinc-300 flex items-center gap-2">
                            <Sparkles className="h-4 w-4" />
                            Type
                          </label>
                          <Select 
                            value={filters.type || FILTER_OPTIONS.ALL_TYPES} 
                            onValueChange={(value) => updateFilter("type", value)}
                          >
                            <SelectTrigger className="bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                              <SelectItem value={FILTER_OPTIONS.ALL_TYPES}>All Types</SelectItem>
                              {CARD_CONFIG.TYPES.map((type) => (
                                <SelectItem key={type} value={type}>
                                  {type.charAt(0).toUpperCase() + type.slice(1)}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>

                        {/* Sort Options */}
                        <div className="space-y-3">
                          <label className="text-sm font-semibold text-zinc-300 flex items-center gap-2">
                            <TrendingUp className="h-4 w-4" />
                            Sort By
                          </label>
                          <Select defaultValue="created_desc">
                            <SelectTrigger className="bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                              <SelectItem value="created_desc">Newest First</SelectItem>
                              <SelectItem value="created_asc">Oldest First</SelectItem>
                              <SelectItem value="name_asc">Name A-Z</SelectItem>
                              <SelectItem value="name_desc">Name Z-A</SelectItem>
                              <SelectItem value="level_desc">Level High-Low</SelectItem>
                              <SelectItem value="level_asc">Level Low-High</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>

                        {/* Clear Filters */}
                        <div className="space-y-3">
                          <label className="text-sm font-semibold text-zinc-300 flex items-center gap-2">
                            <RotateCcw className="h-4 w-4" />
                            Actions
                          </label>
                          <Button
                            variant="outline"
                            onClick={clearFilters}
                            disabled={!hasActiveFilters}
                            className="w-full border-zinc-700/50 hover:bg-zinc-800/50 disabled:opacity-50"
                          >
                            Clear All Filters
                          </Button>
                        </div>
                      </div>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* Enhanced Bulk Actions */}
        <AnimatePresence>
          {selectionCount > 0 && (
            <motion.div
              initial={{ opacity: 0, y: -20, scale: 0.95 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -20, scale: 0.95 }}
              className="sticky top-4 z-40"
            >
              <Card className="bg-gradient-to-r from-pink-900/20 via-purple-900/20 to-cyan-900/20 border-pink-500/30 backdrop-blur-2xl shadow-2xl">
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <motion.div 
                        className="p-3 bg-pink-500/20 rounded-xl"
                        animate={{ rotate: [0, 10, -10, 0] }}
                        transition={{ duration: 2, repeat: Infinity }}
                      >
                        <Heart className="h-6 w-6 text-pink-400" />
                      </motion.div>
                      <div>
                        <p className="font-bold text-xl text-white">
                          {selectionCount} card{selectionCount !== 1 ? 's' : ''} selected
                        </p>
                        <p className="text-sm text-zinc-400">Choose an action to perform on selected cards</p>
                      </div>
                    </div>
                    <div className="flex gap-3">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={handleBulkDelete}
                        disabled={loading}
                        className="border-red-500/50 text-red-400 hover:bg-red-500/10 backdrop-blur-sm"
                      >
                        <Trash2 className="mr-2 h-4 w-4" />
                        Delete Selected
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="border-blue-500/50 text-blue-400 hover:bg-blue-500/10 backdrop-blur-sm"
                      >
                        <Download className="mr-2 h-4 w-4" />
                        Export Selected
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={clearSelection}
                        className="border-zinc-600/50 hover:bg-zinc-800/50 backdrop-blur-sm"
                      >
                        Clear Selection
                      </Button>
                    </div>
                  </div>
                  <div className="mt-4">
                    <Progress value={(selectionCount / cards.length) * 100} className="h-2" />
                  </div>
                </CardContent>
              </Card>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Revolutionary Cards Display */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 1 }}
        >
          <Card className="bg-gradient-to-br from-black/60 via-zinc-900/40 to-black/60 backdrop-blur-2xl border-white/10 shadow-2xl overflow-hidden">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-2xl text-white flex items-center gap-3">
                  <motion.div
                    animate={{ rotate: 360 }}
                    transition={{ duration: 20, repeat: Infinity, ease: "linear" }}
                  >
                    <Database className="h-6 w-6 text-pink-400" />
                  </motion.div>
                  Revolutionary Card Collection
                </CardTitle>
                {cards.length > 0 && (
                  <div className="flex items-center gap-4">
                    <span className="text-sm text-zinc-400">
                      Displaying {cards.length} of {stats.total} cards
                    </span>
                    <div className="flex items-center gap-3">
                      <Checkbox
                        checked={isAllSelected(cards)}
                        onCheckedChange={() => toggleAll(cards)}
                        className="border-white/30 data-[state=checked]:bg-pink-500"
                      />
                      <span className="text-sm text-zinc-400">Select All</span>
                    </div>
                  </div>
                )}
              </div>
            </CardHeader>
            <CardContent className="p-8">
              {loading ? (
                <div className="flex items-center justify-center py-24">
                  <div className="text-center">
                    <motion.div
                      animate={{ rotate: 360 }}
                      transition={{ duration: 1, repeat: Infinity, ease: "linear" }}
                      className="inline-block"
                    >
                      <Sparkles className="h-12 w-12 text-pink-500 mx-auto mb-4" />
                    </motion.div>
                    <p className="text-white font-bold text-xl mb-2">Loading your magical collection...</p>
                    <p className="text-zinc-400">Preparing the most beautiful cards experience</p>
                  </div>
                </div>
              ) : cards.length === 0 ? (
                <motion.div
                  initial={{ opacity: 0, scale: 0.9 }}
                  animate={{ opacity: 1, scale: 1 }}
                  className="text-center py-24"
                >
                  <div className="mb-8">
                    <motion.div 
                      className="mx-auto w-32 h-32 bg-gradient-to-br from-pink-500/20 to-purple-500/20 rounded-full flex items-center justify-center shadow-2xl backdrop-blur-xl border border-white/10"
                      animate={{ 
                        scale: [1, 1.1, 1],
                        rotate: [0, 10, -10, 0]
                      }}
                      transition={{ 
                        duration: 3, 
                        repeat: Infinity,
                        ease: "easeInOut"
                      }}
                    >
                      <Plus className="h-16 w-16 text-pink-400" />
                    </motion.div>
                  </div>
                  <h3 className="text-3xl font-bold text-white mb-4">No cards found</h3>
                  <p className="text-zinc-400 text-lg mb-8 max-w-md mx-auto leading-relaxed">
                    {hasActiveFilters 
                      ? "No cards match your current filters. Try adjusting your search criteria to discover more cards."
                      : "Start building your revolutionary collection by adding your first K-pop trading cards and experience the magic."
                    }
                  </p>
                  <div className="flex flex-col sm:flex-row gap-4 justify-center">
                    {hasActiveFilters ? (
                      <Button
                        variant="outline"
                        onClick={clearFilters}
                        className="border-zinc-700/50 hover:bg-zinc-800/50 backdrop-blur-sm"
                      >
                        <RotateCcw className="mr-2 h-4 w-4" />
                        Clear All Filters
                      </Button>
                    ) : (
                      <>
                        <Button
                          onClick={() => router.push('/dashboard/cards/new')}
                          className="bg-gradient-to-r from-pink-600 via-pink-500 to-purple-600 hover:from-pink-700 hover:via-pink-600 hover:to-purple-700 text-white shadow-2xl shadow-pink-500/25"
                        >
                          <Plus className="mr-2 h-4 w-4" />
                          Add Your First Card
                        </Button>
                        <Button
                          variant="outline"
                          onClick={() => router.push('/dashboard/import')}
                          className="border-zinc-700/50 hover:bg-zinc-800/50 backdrop-blur-sm"
                        >
                          <Upload className="mr-2 h-4 w-4" />
                          Import Album Collection
                        </Button>
                      </>
                    )}
                  </div>
                </motion.div>
              ) : (
                <div className="space-y-8">
                  <AnimatePresence mode="wait">
                    {viewMode === 'masonry' ? (
                      <motion.div
                        key="masonry"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="columns-1 sm:columns-2 lg:columns-3 xl:columns-4 2xl:columns-5 gap-6 space-y-6"
                        style={{ columnWidth: `${cardSize}px` }}
                      >
                        {cards.map((card, index) => (
                          <motion.div
                            key={card.id}
                            initial={{ opacity: 0, y: 20 }}
                            animate={{ opacity: 1, y: 0 }}
                            transition={{ delay: index * 0.05 }}
                            className="break-inside-avoid mb-6"
                          >
                            <OptimizedCard
                              card={card}
                              onAction={handleCardAction}
                              isSelected={isSelected(card.id.toString())}
                              onToggle={() => toggleCard(card.id.toString())}
                            />
                          </motion.div>
                        ))}
                      </motion.div>
                    ) : viewMode === 'grid' ? (
                      <motion.div
                        key="grid"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 gap-8"
                      >
                        {cards.map((card, index) => (
                          <motion.div
                            key={card.id}
                            initial={{ opacity: 0, scale: 0.8 }}
                            animate={{ opacity: 1, scale: 1 }}
                            transition={{ delay: index * 0.05 }}
                          >
                            <OptimizedCard
                              card={card}
                              onAction={handleCardAction}
                              isSelected={isSelected(card.id.toString())}
                              onToggle={() => toggleCard(card.id.toString())}
                            />
                          </motion.div>
                        ))}
                      </motion.div>
                    ) : (
                      <motion.div
                        key="list"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="space-y-4"
                      >
                        {cards.map((card, index) => (
                          <motion.div
                            key={card.id}
                            initial={{ opacity: 0, x: -20 }}
                            animate={{ opacity: 1, x: 0 }}
                            transition={{ delay: index * 0.03 }}
                          >
                            <Card className="bg-black/40 backdrop-blur-xl border-white/10 hover:border-white/20 transition-all duration-300 group overflow-hidden">
                              <CardContent className="p-6">
                                <div className="flex items-center gap-6">
                                  <Checkbox
                                    checked={isSelected(card.id.toString())}
                                    onCheckedChange={() => toggleCard(card.id.toString())}
                                    className="border-white/30 data-[state=checked]:bg-pink-500"
                                  />
                                  
                                  <div className="w-20 h-28 bg-zinc-800 rounded-lg overflow-hidden flex-shrink-0 group-hover:shadow-lg transition-shadow">
                                    {card.image_url ? (
                                      <img src={card.image_url} alt={card.name} className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300" />
                                    ) : (
                                      <div className="w-full h-full bg-zinc-700 flex items-center justify-center">
                                        <Database className="h-8 w-8 text-zinc-500" />
                                      </div>
                                    )}
                                  </div>

                                  <div className="flex-1 min-w-0 space-y-2">
                                    <h3 className="font-bold text-xl text-white truncate group-hover:text-pink-300 transition-colors">
                                      {card.name}
                                    </h3>
                                    <p className="text-zinc-400 truncate">{card.collection_name}</p>
                                    <div className="flex items-center gap-3">
                                      <Badge className={`${getLevelBadgeClasses(card.level)} shadow-lg`}>
                                        Level {card.level}
                                      </Badge>
                                      {card.animated && (
                                        <Badge className="bg-pink-500/20 text-pink-300 border-pink-500/30">
                                          <Sparkles className="mr-1 h-3 w-3" />
                                          Animated
                                        </Badge>
                                      )}
                                      {card.promo && (
                                        <Badge className="bg-orange-500/20 text-orange-300 border-orange-500/30">
                                          <Flame className="mr-1 h-3 w-3" />
                                          Promo
                                        </Badge>
                                      )}
                                    </div>
                                  </div>

                                  <div className="flex items-center gap-3">
                                    <span className="text-xs text-zinc-500 flex items-center gap-1">
                                      <Clock className="h-3 w-3" />
                                      {new Date(card.created_at).toLocaleDateString()}
                                    </span>
                                    <DropdownMenu>
                                      <DropdownMenuTrigger asChild>
                                        <Button variant="ghost" size="sm" className="h-10 w-10 p-0 hover:bg-white/10">
                                          <MoreVertical className="h-4 w-4" />
                                        </Button>
                                      </DropdownMenuTrigger>
                                      <DropdownMenuContent align="end" className="w-56 bg-black/90 backdrop-blur-xl border-white/10">
                                        {[
                                          { icon: Eye, label: "View Details", action: "view" },
                                          { icon: Edit, label: "Edit Card", action: "edit" },
                                          { icon: Copy, label: "Duplicate", action: "duplicate" },
                                          { icon: Download, label: "Download", action: "download" },
                                          null,
                                          { icon: Trash2, label: "Delete", action: "delete", destructive: true },
                                        ].map((item, index) => 
                                          item === null ? (
                                            <DropdownMenuSeparator key={index} className="bg-white/10" />
                                          ) : (
                                            <DropdownMenuItem 
                                              key={item.action}
                                              onClick={() => handleCardAction(item.action, card)}
                                              className={`cursor-pointer ${
                                                item.destructive ? 'text-red-400 focus:text-red-300' : 'text-white focus:text-white'
                                              }`}
                                            >
                                              <item.icon className="mr-3 h-4 w-4" />
                                              {item.label}
                                            </DropdownMenuItem>
                                          )
                                        )}
                                      </DropdownMenuContent>
                                    </DropdownMenu>
                                  </div>
                                </div>
                              </CardContent>
                            </Card>
                          </motion.div>
                        ))}
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              )}
            </CardContent>
          </Card>
        </motion.div>
      </div>

      {/* Floating Action Button */}
      <motion.div
        initial={{ scale: 0 }}
        animate={{ scale: 1 }}
        transition={{ delay: 1.5 }}
        className="fixed bottom-8 right-8 z-50"
      >
        <Button
          onClick={() => router.push('/dashboard/cards/new')}
          size="lg"
          className="h-16 w-16 rounded-full bg-gradient-to-r from-pink-600 to-purple-600 hover:from-pink-700 hover:to-purple-700 shadow-2xl shadow-pink-500/50 hover:shadow-pink-500/75 transition-all duration-300"
        >
          <Plus className="h-8 w-8" />
        </Button>
      </motion.div>
    </div>
  );
}