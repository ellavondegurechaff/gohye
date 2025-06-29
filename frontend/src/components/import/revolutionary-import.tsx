"use client";

import { useState, useCallback, useMemo } from "react";
import { useRouter } from "next/navigation";
import { motion, AnimatePresence } from "framer-motion";
import { toast } from "sonner";
import { useDropzone } from "react-dropzone";

// UI Components
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Checkbox } from "@/components/ui/checkbox";

// Icons
import { 
  Upload, FileImage, FolderOpen, Sparkles, Zap, Target, Crown,
  Download, RefreshCw, Check, X, AlertCircle, Info, Star,
  Image as ImageIcon, Database, Users, BarChart3, Clock,
  ArrowRight, ArrowLeft, Plus, Trash2, Eye, Edit, Wand2,
  Layers, Palette, Move3D, Volume2, VolumeX, Settings
} from "lucide-react";

// Types
import type { CollectionDTO } from "@/lib/types";
import { apiClient } from "@/lib/api";

// Enhanced import types
interface ValidationError {
  file_name: string;
  error_type: string;
  description: string;
  severity: string;
  suggestion?: string;
}

interface ImportResult {
  collection_id: string;
  collection_created: boolean;
  cards_created: number;
  cards_skipped: number;
  cards_updated: number;
  first_card_id?: number;
  last_card_id?: number;
  files_uploaded: string[];
  files_skipped: string[];
  validation_errors: ValidationError[];
  processing_errors: any[];
  success: boolean;
  partial_success: boolean;
  error_message?: string;
  processing_time_ms: number;
  import_summary?: {
    total_files: number;
    valid_files: number;
    invalid_files: number;
    processed_files: number;
    failed_files: number;
    level_stats: Record<number, number>;
    file_type_stats: Record<string, number>;
    duplicates: string[];
    large_files: string[];
  };
}

interface RevolutionaryImportProps {
  collections: CollectionDTO[];
}

interface ImportedFile {
  id: string;
  file: File;
  preview: string;
  name: string;
  level: number;
  animated: boolean;
  tags: string[];
  collection_id: string;
  status: 'pending' | 'processing' | 'success' | 'error';
  error?: string;
}

export function RevolutionaryImport({ collections }: RevolutionaryImportProps) {
  const router = useRouter();
  const [currentStep, setCurrentStep] = useState(1);
  const [importMode, setImportMode] = useState<'existing' | 'new'>('existing');
  const [selectedCollection, setSelectedCollection] = useState<string>("");
  const [newCollectionData, setNewCollectionData] = useState({
    name: "",
    description: "",
    collection_type: "girl_group" as const,
  });
  const [importedFiles, setImportedFiles] = useState<ImportedFile[]>([]);
  const [isImporting, setIsImporting] = useState(false);
  const [isValidating, setIsValidating] = useState(false);
  const [importProgress, setImportProgress] = useState(0);
  const [soundEnabled, setSoundEnabled] = useState(true);
  const [autoDetection, setAutoDetection] = useState({
    level: true,
    animated: true,
    tags: true,
  });
  
  // Enhanced validation and import state
  const [validationErrors, setValidationErrors] = useState<ValidationError[]>([]);
  const [importResult, setImportResult] = useState<ImportResult | null>(null);
  const [overwriteMode, setOverwriteMode] = useState<'skip' | 'overwrite' | 'update'>('skip');
  const [createCollection, setCreateCollection] = useState(false);

  // Dropzone configuration
  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    accept: {
      'image/*': ['.jpeg', '.jpg', '.png', '.gif', '.webp']
    },
    multiple: true,
    onDrop: useCallback((acceptedFiles: File[]) => {
      const newFiles = acceptedFiles.map((file, index) => ({
        id: `${Date.now()}-${index}`,
        file,
        preview: URL.createObjectURL(file),
        name: file.name.replace(/\.[^/.]+$/, ""),
        level: autoDetection.level ? detectLevel(file.name) : 1,
        animated: autoDetection.animated ? detectAnimated(file.name) : false,
        tags: autoDetection.tags ? detectTags(file.name) : [],
        collection_id: selectedCollection,
        status: 'pending' as const,
      }));

      setImportedFiles(prev => [...prev, ...newFiles]);
      
      if (soundEnabled) {
        // Play upload sound
        const audio = new Audio('data:audio/wav;base64,UklGRnoGAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQoGAACBhYqFbF1fdJivrJBhNjVgodDbq2EcBj+a2/LDciUFLIHO8tiJNwgZaLvt559NEAxQp+PwtmMcBjiR1/LMeSwFJHfH8N2QQAoUXrTp66hVFApGn+DyvmEXBze/zPLHdSMELIHO8tuFNwgZZ7zs5ZdMEAxQp+PwtmUcBzaRz+3PgykEJ2+66rBuFgU7hM7y2YU3CBlkvuzjl0wQDFCn4/C2ZRwGNZHP7K+DIggudM7u3IU7CRZq6uW7fBYDMn/P8N2NQAoTXrTp66hVFApPiOLxy2Y/CzODwcVrPAq/hM7y2YU4CRNlu+zhnUsQDFCn4/C2ZRwHNZPO7K+DIgcme82+dRUFNnfM8N+QQAoUWqzn6a5hFgo0Vn/C7r1bGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJtteKdVxYKRoPH6bRmHAc5hM7y2YU3CRNluuzjmE4QDFCn4/C2ZRwHNZLO7K+DIgcme82+dRUFN3zK8N+QQQsUWqzn6a5hFgo0Vn/C7r1bGgwzf8nw34Y/CiaAyPDajTsIG2e86qxCFwRGiuvyv2gaCj2AzPLHdCcEKne66K2DIgctjMrt5ZNIDR5ryPHRhzIJGF615OOiSQwQRa3l8bdhFgo2k8/sz4EqBypWyuqjTgwRSarl8bdhFgo2k8/sz4EpBipwyuqgTAsMSaq68LRjFgo2k8/sz4AkBipwyuqgTAsMSaq68LRjFgo2k8/sz4ApBipwyuqgTAwQD15q9+G2ZRwHNZLP7M+DIgctdM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKe=');
        audio.volume = 0.2;
        audio.play().catch(() => {});
      }
    }, [selectedCollection, autoDetection, soundEnabled]),
  });

  // AI-powered detection functions
  const detectLevel = (filename: string): number => {
    const name = filename.toLowerCase();
    if (name.includes('rare') || name.includes('special') || name.includes('5')) return 5;
    if (name.includes('legendary') || name.includes('4')) return 4;
    if (name.includes('epic') || name.includes('3')) return 3;
    if (name.includes('uncommon') || name.includes('2')) return 2;
    return 1;
  };

  const detectAnimated = (filename: string): boolean => {
    const name = filename.toLowerCase();
    return name.includes('animated') || name.includes('gif') || name.includes('motion');
  };

  const detectTags = (filename: string): string[] => {
    const name = filename.toLowerCase();
    const tags: string[] = [];
    
    if (name.includes('photo')) tags.push('photocard');
    if (name.includes('sign')) tags.push('signed');
    if (name.includes('holo')) tags.push('holographic');
    if (name.includes('limited')) tags.push('limited');
    if (name.includes('pre')) tags.push('pre-order');
    
    return tags;
  };

  // Step navigation
  const steps = [
    { number: 1, title: "Collection Setup", description: "Choose or create collection" },
    { number: 2, title: "File Upload", description: "Upload and configure cards" },
    { number: 3, title: "Review & Import", description: "Finalize and import cards" },
  ];

  const nextStep = () => {
    if (currentStep < 3) {
      setCurrentStep(currentStep + 1);
    }
  };

  const prevStep = () => {
    if (currentStep > 1) {
      setCurrentStep(currentStep - 1);
    }
  };

  const canProceedToStep2 = useMemo(() => {
    if (importMode === 'existing') {
      return selectedCollection !== "";
    } else {
      return newCollectionData.name.trim() !== "" && newCollectionData.collection_type.trim() !== "";
    }
  }, [importMode, selectedCollection, newCollectionData]);

  const canProceedToStep3 = useMemo(() => {
    return importedFiles.length > 0;
  }, [importedFiles]);

  // Enhanced validation function
  const handleValidation = async () => {
    if (importedFiles.length === 0) return;

    setIsValidating(true);
    setValidationErrors([]);

    try {
      const formData = new FormData();
      
      // Add form fields
      formData.append('collection_id', importMode === 'existing' ? selectedCollection : newCollectionData.name.toLowerCase().replace(/\s+/g, '_'));
      formData.append('display_name', importMode === 'existing' ? (collections.find(c => c.id === selectedCollection)?.name || '') : newCollectionData.name);
      formData.append('group_type', importMode === 'existing' ? 'girlgroups' : (newCollectionData.collection_type === 'girl_group' ? 'girlgroups' : 'boygroups'));
      formData.append('is_promo', 'false');
      formData.append('create_collection', createCollection.toString());
      formData.append('overwrite_mode', overwriteMode);
      
      // Add files
      importedFiles.forEach((file, index) => {
        formData.append('files', file.file);
      });

      // Call validation endpoint
      const response = await fetch('/api/cards/import/validate', {
        method: 'POST',
        body: formData,
      });

      const result = await response.json();
      
      if (result.success && result.data) {
        setValidationErrors(result.data.validation_errors || []);
        
        if (result.data.validation_errors?.length > 0) {
          const criticalErrors = result.data.validation_errors.filter((e: ValidationError) => e.severity === 'critical');
          if (criticalErrors.length > 0) {
            toast.error(`Found ${criticalErrors.length} critical validation errors`);
          } else {
            toast.warning(`Found ${result.data.validation_errors.length} validation warnings`);
          }
        } else {
          toast.success('All files passed validation!');
        }
      } else {
        throw new Error(result.message || 'Validation failed');
      }
    } catch (error: any) {
      console.error('Validation failed:', error);
      toast.error('Validation failed: ' + error.message);
    } finally {
      setIsValidating(false);
    }
  };

  // Enhanced import process
  const handleImport = async () => {
    if (importedFiles.length === 0) return;

    setIsImporting(true);
    setImportProgress(0);
    setImportResult(null);

    try {
      const formData = new FormData();
      
      // Add form fields
      formData.append('collection_id', importMode === 'existing' ? selectedCollection : newCollectionData.name.toLowerCase().replace(/\s+/g, '_'));
      formData.append('display_name', importMode === 'existing' ? (collections.find(c => c.id === selectedCollection)?.name || '') : newCollectionData.name);
      formData.append('group_type', importMode === 'existing' ? 'girlgroups' : (newCollectionData.collection_type === 'girl_group' ? 'girlgroups' : 'boygroups'));
      formData.append('is_promo', 'false');
      formData.append('create_collection', (importMode === 'new' || createCollection).toString());
      formData.append('overwrite_mode', overwriteMode);
      
      // Add files
      importedFiles.forEach((file, index) => {
        formData.append('files', file.file);
      });

      // Call import endpoint
      const response = await fetch('/api/cards/import', {
        method: 'POST',
        body: formData,
      });

      const result = await response.json();
      
      if (result.success && result.data) {
        const importData: ImportResult = result.data;
        setImportResult(importData);
        
        // Update file statuses based on results
        setImportedFiles(prev => prev.map(file => {
          if (importData.files_uploaded.includes(file.file.name)) {
            return { ...file, status: 'success' };
          } else if (importData.files_skipped.includes(file.file.name)) {
            return { ...file, status: 'pending', error: 'Skipped' };
          } else {
            return { ...file, status: 'error', error: 'Processing failed' };
          }
        }));

        setImportProgress(100);

        if (importData.success) {
          toast.success(`Successfully imported ${importData.cards_created} cards!`);
          
          // Play success sound
          if (soundEnabled) {
            const audio = new Audio('data:audio/wav;base64,UklGRhAGAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YewFAACpqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq');
            audio.volume = 0.3;
            audio.play().catch(() => {});
          }
          
          // Redirect to collection after delay
          setTimeout(() => {
            router.push(`/dashboard/cards?collection=${importData.collection_id}`);
          }, 3000);
        } else if (importData.partial_success) {
          toast.warning(`Partially imported: ${importData.cards_created} succeeded, ${importData.processing_errors.length} failed`);
        } else {
          throw new Error(importData.error_message || 'Import failed');
        }
      } else {
        throw new Error(result.message || 'Import failed');
      }

    } catch (error: any) {
      console.error('Import failed:', error);
      toast.error('Import failed: ' + error.message);
      setImportResult(null);
    } finally {
      setIsImporting(false);
    }
  };

  const removeFile = (fileId: string) => {
    setImportedFiles(prev => prev.filter(f => f.id !== fileId));
  };

  const updateFile = (fileId: string, updates: Partial<ImportedFile>) => {
    setImportedFiles(prev => prev.map(f => 
      f.id === fileId ? { ...f, ...updates } : f
    ));
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900/50 to-black relative overflow-hidden">
      {/* Animated Background */}
      <div className="absolute inset-0 opacity-20">
        <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-purple-500/10 rounded-full blur-3xl animate-pulse" />
        <div className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-cyan-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '2s' }} />
        <div className="absolute top-1/2 left-1/2 w-96 h-96 bg-pink-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '4s' }} />
      </div>

      <div className="relative z-10 container mx-auto px-6 py-8 space-y-8">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: -30 }}
          animate={{ opacity: 1, y: 0 }}
          className="text-center space-y-6"
        >
          <div className="space-y-4">
            <h1 className="text-5xl font-black bg-gradient-to-r from-white via-purple-200 to-cyan-200 bg-clip-text text-transparent">
              Import Wizard
            </h1>
            <p className="text-xl text-zinc-400 max-w-2xl mx-auto">
              Effortlessly import your K-pop card collections with AI-powered detection
            </p>
          </div>

          {/* Sound Toggle */}
          <div className="flex justify-center">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSoundEnabled(!soundEnabled)}
              className="text-zinc-400 hover:text-white"
            >
              {soundEnabled ? <Volume2 className="h-4 w-4 mr-2" /> : <VolumeX className="h-4 w-4 mr-2" />}
              {soundEnabled ? 'Sound On' : 'Sound Off'}
            </Button>
          </div>
        </motion.div>

        {/* Progress Steps */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
        >
          <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
            <CardContent className="p-8">
              <div className="flex items-center justify-between">
                {steps.map((step, index) => (
                  <div key={step.number} className="flex items-center">
                    <div className="flex items-center">
                      <motion.div
                        className={`w-12 h-12 rounded-full flex items-center justify-center border-2 transition-all duration-300 ${
                          currentStep >= step.number
                            ? 'bg-gradient-to-r from-purple-500 to-cyan-500 border-transparent text-white'
                            : 'border-zinc-600 text-zinc-400'
                        }`}
                        animate={currentStep === step.number ? { scale: [1, 1.1, 1] } : {}}
                        transition={{ duration: 0.5 }}
                      >
                        {currentStep > step.number ? (
                          <Check className="h-6 w-6" />
                        ) : (
                          <span className="font-bold">{step.number}</span>
                        )}
                      </motion.div>
                      <div className="ml-4 text-left">
                        <h3 className={`font-semibold transition-colors ${
                          currentStep >= step.number ? 'text-white' : 'text-zinc-400'
                        }`}>
                          {step.title}
                        </h3>
                        <p className="text-sm text-zinc-500">{step.description}</p>
                      </div>
                    </div>
                    {index < steps.length - 1 && (
                      <div className={`mx-8 h-0.5 w-24 transition-colors ${
                        currentStep > step.number ? 'bg-gradient-to-r from-purple-500 to-cyan-500' : 'bg-zinc-700'
                      }`} />
                    )}
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* Step Content */}
        <AnimatePresence mode="wait">
          {currentStep === 1 && (
            <motion.div
              key="step1"
              initial={{ opacity: 0, x: 50 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -50 }}
              className="space-y-6"
            >
              <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
                <CardHeader>
                  <CardTitle className="text-2xl text-white flex items-center gap-3">
                    <FolderOpen className="h-8 w-8 text-purple-400" />
                    Collection Setup
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-6">
                  {/* Import Mode Selection */}
                  <Tabs value={importMode} onValueChange={(value) => setImportMode(value as any)}>
                    <TabsList className="grid w-full grid-cols-2 bg-black/60 backdrop-blur-xl">
                      <TabsTrigger value="existing" className="data-[state=active]:bg-zinc-700/50">
                        Use Existing Collection
                      </TabsTrigger>
                      <TabsTrigger value="new" className="data-[state=active]:bg-zinc-700/50">
                        Create New Collection
                      </TabsTrigger>
                    </TabsList>
                    
                    <TabsContent value="existing" className="space-y-4">
                      <div className="space-y-3">
                        <label className="text-sm font-medium text-zinc-300">Select Collection</label>
                        <Select value={selectedCollection} onValueChange={setSelectedCollection}>
                          <SelectTrigger className="h-12 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                            <SelectValue placeholder="Choose an existing collection..." />
                          </SelectTrigger>
                          <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                            {collections.map((collection) => (
                              <SelectItem key={collection.id} value={collection.id}>
                                <div className="flex items-center gap-3">
                                  <div className={`w-3 h-3 rounded-full ${
                                    collection.collection_type === 'girl_group' ? 'bg-pink-500' :
                                    collection.collection_type === 'boy_group' ? 'bg-blue-500' :
                                    'bg-zinc-500'
                                  }`} />
                                  <span>{collection.name}</span>
                                  <Badge variant="outline" className="ml-auto text-xs">
                                    {collection.card_count || 0} cards
                                  </Badge>
                                </div>
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </TabsContent>
                    
                    <TabsContent value="new" className="space-y-4">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="space-y-3">
                          <label className="text-sm font-medium text-zinc-300">Collection Name</label>
                          <Input
                            value={newCollectionData.name}
                            onChange={(e) => setNewCollectionData(prev => ({ ...prev, name: e.target.value }))}
                            placeholder="Enter collection name..."
                            className="h-12 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white"
                          />
                        </div>
                        <div className="space-y-3">
                          <label className="text-sm font-medium text-zinc-300">Collection Type</label>
                          <Select 
                            value={newCollectionData.collection_type} 
                            onValueChange={(value: any) => setNewCollectionData(prev => ({ ...prev, collection_type: value }))}
                          >
                            <SelectTrigger className="h-12 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                              <SelectItem value="girl_group">Girl Group</SelectItem>
                              <SelectItem value="boy_group">Boy Group</SelectItem>
                              <SelectItem value="other">Other</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </div>
                      <div className="space-y-3">
                        <label className="text-sm font-medium text-zinc-300">Description (Optional)</label>
                        <Textarea
                          value={newCollectionData.description}
                          onChange={(e) => setNewCollectionData(prev => ({ ...prev, description: e.target.value }))}
                          placeholder="Describe your collection..."
                          className="min-h-[100px] bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white resize-none"
                        />
                      </div>
                    </TabsContent>
                  </Tabs>

                  {/* Enhanced Import Settings */}
                  <div className="mt-6 space-y-4">
                    <h3 className="text-lg font-semibold text-white flex items-center gap-2">
                      <Settings className="h-5 w-5 text-purple-400" />
                      Import Settings
                    </h3>
                    
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div className="space-y-3">
                        <label className="text-sm font-medium text-zinc-300">Overwrite Mode</label>
                        <Select value={overwriteMode} onValueChange={(value: any) => setOverwriteMode(value)}>
                          <SelectTrigger className="h-10 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                            <SelectItem value="skip">Skip existing cards</SelectItem>
                            <SelectItem value="overwrite">Overwrite existing cards</SelectItem>
                            <SelectItem value="update">Update existing cards</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      
                      <div className="flex items-center space-x-3 p-3 rounded-lg bg-zinc-800/30">
                        <Checkbox
                          checked={createCollection}
                          onCheckedChange={(checked) => setCreateCollection(checked as boolean)}
                          className="border-white/30 data-[state=checked]:bg-purple-500"
                        />
                        <div>
                          <p className="text-sm font-medium text-white">Auto-create collection</p>
                          <p className="text-xs text-zinc-400">Create collection if it doesn't exist</p>
                        </div>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </motion.div>
          )}

          {currentStep === 2 && (
            <motion.div
              key="step2"
              initial={{ opacity: 0, x: 50 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -50 }}
              className="space-y-6"
            >
              {/* Auto-Detection Settings */}
              <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
                <CardHeader>
                  <CardTitle className="text-xl text-white flex items-center gap-3">
                    <Wand2 className="h-6 w-6 text-cyan-400" />
                    AI-Powered Detection
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    {[
                      { key: 'level', label: 'Auto-detect Level', description: 'Analyze filenames for rarity hints' },
                      { key: 'animated', label: 'Auto-detect Animated', description: 'Identify animated cards from filenames' },
                      { key: 'tags', label: 'Auto-detect Tags', description: 'Extract tags from filename patterns' },
                    ].map((option) => (
                      <div key={option.key} className="flex items-center space-x-3 p-4 rounded-lg bg-zinc-800/30 hover:bg-zinc-800/50 transition-colors">
                        <Checkbox
                          checked={autoDetection[option.key as keyof typeof autoDetection]}
                          onCheckedChange={(checked) => 
                            setAutoDetection(prev => ({ ...prev, [option.key]: checked as boolean }))
                          }
                          className="border-white/30 data-[state=checked]:bg-cyan-500"
                        />
                        <div>
                          <p className="text-sm font-medium text-white">{option.label}</p>
                          <p className="text-xs text-zinc-400">{option.description}</p>
                        </div>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>

              {/* File Upload Area */}
              <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
                <CardHeader>
                  <CardTitle className="text-2xl text-white flex items-center gap-3">
                    <Upload className="h-8 w-8 text-purple-400" />
                    Upload Card Images
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div
                    {...getRootProps()}
                    className={`border-2 border-dashed rounded-xl p-12 text-center transition-all duration-300 cursor-pointer ${
                      isDragActive
                        ? 'border-purple-500 bg-purple-500/10'
                        : 'border-zinc-600 hover:border-purple-500/50 hover:bg-purple-500/5'
                    }`}
                  >
                    <input {...getInputProps()} />
                    <motion.div
                      animate={isDragActive ? { scale: 1.05 } : { scale: 1 }}
                      className="space-y-4"
                    >
                      <div className="mx-auto w-20 h-20 bg-gradient-to-br from-purple-500 to-cyan-500 rounded-full flex items-center justify-center">
                        <Upload className="h-10 w-10 text-white" />
                      </div>
                      <div>
                        <p className="text-xl font-semibold text-white mb-2">
                          {isDragActive ? 'Drop files here!' : 'Drag & drop images here'}
                        </p>
                        <p className="text-zinc-400">
                          or <span className="text-purple-400 underline">click to browse</span>
                        </p>
                        <p className="text-sm text-zinc-500 mt-2">
                          Supports: JPEG, PNG, GIF, WebP (Max 10MB each)
                        </p>
                      </div>
                    </motion.div>
                  </div>
                </CardContent>
              </Card>

              {/* Validation Section */}
              {importedFiles.length > 0 && (
                <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
                  <CardHeader>
                    <CardTitle className="text-xl text-white flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <Wand2 className="h-6 w-6 text-cyan-400" />
                        File Validation
                      </div>
                      <Button
                        onClick={handleValidation}
                        disabled={isValidating || importedFiles.length === 0}
                        className="bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-700 hover:to-blue-700 text-white"
                      >
                        {isValidating ? (
                          <>
                            <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                            Validating...
                          </>
                        ) : (
                          <>
                            <Check className="mr-2 h-4 w-4" />
                            Validate Files
                          </>
                        )}
                      </Button>
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    {validationErrors.length > 0 && (
                      <div className="space-y-3">
                        <div className="flex items-center gap-2 mb-4">
                          <AlertCircle className="h-5 w-5 text-amber-400" />
                          <span className="text-white font-medium">
                            Found {validationErrors.length} validation issues
                          </span>
                        </div>
                        <div className="max-h-40 overflow-y-auto space-y-2">
                          {validationErrors.map((error, index) => (
                            <div
                              key={index}
                              className={`p-3 rounded-lg border-l-4 ${
                                error.severity === 'critical'
                                  ? 'bg-red-900/20 border-red-500'
                                  : error.severity === 'high'
                                  ? 'bg-orange-900/20 border-orange-500'
                                  : error.severity === 'medium'
                                  ? 'bg-yellow-900/20 border-yellow-500'
                                  : 'bg-blue-900/20 border-blue-500'
                              }`}
                            >
                              <div className="flex items-start gap-2">
                                <div className="flex-1">
                                  <p className="text-sm font-medium text-white">{error.file_name}</p>
                                  <p className="text-sm text-zinc-300">{error.description}</p>
                                  {error.suggestion && (
                                    <p className="text-xs text-zinc-400 mt-1">💡 {error.suggestion}</p>
                                  )}
                                </div>
                                <Badge
                                  variant="outline"
                                  className={
                                    error.severity === 'critical'
                                      ? 'border-red-500 text-red-400'
                                      : error.severity === 'high'
                                      ? 'border-orange-500 text-orange-400'
                                      : error.severity === 'medium'
                                      ? 'border-yellow-500 text-yellow-400'
                                      : 'border-blue-500 text-blue-400'
                                  }
                                >
                                  {error.severity}
                                </Badge>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                    {validationErrors.length === 0 && importedFiles.length > 0 && (
                      <div className="text-center py-4">
                        <div className="inline-flex items-center gap-2 text-green-400">
                          <Check className="h-5 w-5" />
                          <span>Click "Validate Files" to check for issues before importing</span>
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>
              )}

              {/* Uploaded Files */}
              {importedFiles.length > 0 && (
                <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
                  <CardHeader>
                    <CardTitle className="text-xl text-white flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <FileImage className="h-6 w-6 text-cyan-400" />
                        Uploaded Files ({importedFiles.length})
                      </div>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setImportedFiles([])}
                        className="border-red-500/50 text-red-400 hover:bg-red-500/10"
                      >
                        <Trash2 className="h-4 w-4 mr-2" />
                        Clear All
                      </Button>
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 max-h-96 overflow-y-auto">
                      {importedFiles.map((file) => (
                        <motion.div
                          key={file.id}
                          layout
                          initial={{ opacity: 0, scale: 0.9 }}
                          animate={{ opacity: 1, scale: 1 }}
                          className="flex items-center gap-4 p-4 bg-zinc-800/30 rounded-lg hover:bg-zinc-800/50 transition-colors"
                        >
                          <div className="w-16 h-20 bg-zinc-700 rounded overflow-hidden flex-shrink-0">
                            <img
                              src={file.preview}
                              alt={file.name}
                              className="w-full h-full object-cover"
                            />
                          </div>
                          <div className="flex-1 min-w-0 space-y-2">
                            <Input
                              value={file.name}
                              onChange={(e) => updateFile(file.id, { name: e.target.value })}
                              className="text-sm bg-zinc-700/50 border-zinc-600"
                            />
                            <div className="flex items-center gap-2">
                              <Select
                                value={file.level.toString()}
                                onValueChange={(value) => updateFile(file.id, { level: parseInt(value) })}
                              >
                                <SelectTrigger className="w-20 h-8 text-xs bg-zinc-700/50 border-zinc-600">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent className="bg-zinc-800 border-zinc-600">
                                  {[1, 2, 3, 4, 5].map(level => (
                                    <SelectItem key={level} value={level.toString()}>L{level}</SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                              <div className="flex items-center gap-1">
                                <Checkbox
                                  checked={file.animated}
                                  onCheckedChange={(checked) => updateFile(file.id, { animated: checked as boolean })}
                                  className="border-white/30 data-[state=checked]:bg-pink-500"
                                />
                                <span className="text-xs text-zinc-400">Animated</span>
                              </div>
                            </div>
                          </div>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => removeFile(file.id)}
                            className="text-red-400 hover:text-red-300 h-8 w-8 p-0"
                          >
                            <X className="h-4 w-4" />
                          </Button>
                        </motion.div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              )}
            </motion.div>
          )}

          {currentStep === 3 && (
            <motion.div
              key="step3"
              initial={{ opacity: 0, x: 50 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -50 }}
              className="space-y-6"
            >
              <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
                <CardHeader>
                  <CardTitle className="text-2xl text-white flex items-center gap-3">
                    <Target className="h-8 w-8 text-green-400" />
                    Review & Import
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-6">
                  {/* Enhanced Import Summary */}
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    {[
                      { label: "Total Files", value: importedFiles.length, icon: FileImage, color: "text-purple-400" },
                      { label: "Collection", value: importMode === 'new' ? newCollectionData.name : collections.find(c => c.id === selectedCollection)?.name || 'Unknown', icon: FolderOpen, color: "text-cyan-400" },
                      { label: "Animated Cards", value: importedFiles.filter(f => f.animated).length, icon: Sparkles, color: "text-pink-400" },
                      { label: "Validation Issues", value: validationErrors.length, icon: AlertCircle, color: validationErrors.length > 0 ? "text-amber-400" : "text-green-400" },
                    ].map((stat, index) => (
                      <div key={stat.label} className="p-4 bg-zinc-800/30 rounded-lg text-center hover:bg-zinc-800/50 transition-colors">
                        <stat.icon className={`h-6 w-6 ${stat.color} mx-auto mb-2`} />
                        <p className="text-xl font-bold text-white">{stat.value}</p>
                        <p className="text-xs text-zinc-400">{stat.label}</p>
                      </div>
                    ))}
                  </div>

                  {/* Import Results */}
                  {importResult && (
                    <div className="space-y-4">
                      <div className="flex items-center gap-3">
                        <div className={`w-3 h-3 rounded-full ${
                          importResult.success ? 'bg-green-500' : 
                          importResult.partial_success ? 'bg-yellow-500' : 'bg-red-500'
                        }`} />
                        <h3 className="text-lg font-semibold text-white">
                          {importResult.success ? 'Import Successful!' : 
                           importResult.partial_success ? 'Partial Import Success' : 'Import Failed'}
                        </h3>
                      </div>
                      
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                        {[
                          { label: "Cards Created", value: importResult.cards_created, color: "text-green-400" },
                          { label: "Cards Skipped", value: importResult.cards_skipped, color: "text-yellow-400" },
                          { label: "Cards Updated", value: importResult.cards_updated, color: "text-blue-400" },
                          { label: "Processing Time", value: `${importResult.processing_time_ms}ms`, color: "text-purple-400" },
                        ].map((stat) => (
                          <div key={stat.label} className="p-3 bg-black/30 rounded-lg text-center">
                            <p className={`text-lg font-bold ${stat.color}`}>{stat.value}</p>
                            <p className="text-xs text-zinc-400">{stat.label}</p>
                          </div>
                        ))}
                      </div>

                      {importResult.import_summary && (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                          <div className="p-4 bg-zinc-800/30 rounded-lg">
                            <h4 className="text-sm font-semibold text-white mb-3">Level Distribution</h4>
                            <div className="space-y-2">
                              {Object.entries(importResult.import_summary.level_stats).map(([level, count]) => (
                                <div key={level} className="flex items-center justify-between">
                                  <span className="text-sm text-zinc-300">Level {level}</span>
                                  <div className="flex items-center gap-2">
                                    <div className="w-20 h-2 bg-zinc-700 rounded-full overflow-hidden">
                                      <div 
                                        className="h-full bg-gradient-to-r from-purple-500 to-cyan-500"
                                        style={{ width: `${(count / importedFiles.length) * 100}%` }}
                                      />
                                    </div>
                                    <span className="text-sm text-white font-medium w-6">{count}</span>
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                          
                          <div className="p-4 bg-zinc-800/30 rounded-lg">
                            <h4 className="text-sm font-semibold text-white mb-3">File Types</h4>
                            <div className="space-y-2">
                              {Object.entries(importResult.import_summary.file_type_stats).map(([type, count]) => (
                                <div key={type} className="flex items-center justify-between">
                                  <span className="text-sm text-zinc-300">{type.toUpperCase()}</span>
                                  <span className="text-sm text-white font-medium">{count}</span>
                                </div>
                              ))}
                            </div>
                          </div>
                        </div>
                      )}
                    </div>
                  )}

                  {/* Import Progress */}
                  {isImporting && (
                    <div className="space-y-4">
                      <div className="flex items-center justify-between">
                        <span className="text-white font-medium">Import Progress</span>
                        <span className="text-purple-400">{Math.round(importProgress)}%</span>
                      </div>
                      <Progress value={importProgress} className="h-2" />
                      <div className="text-center">
                        <p className="text-sm text-zinc-400">
                          Processing {importedFiles.filter(f => f.status === 'processing').length} files...
                        </p>
                      </div>
                    </div>
                  )}

                  {/* File Status List */}
                  <div className="max-h-60 overflow-y-auto space-y-2">
                    {importedFiles.map((file) => (
                      <div key={file.id} className="flex items-center gap-3 p-3 bg-zinc-800/30 rounded-lg">
                        <div className="w-8 h-10 bg-zinc-700 rounded overflow-hidden">
                          <img src={file.preview} alt={file.name} className="w-full h-full object-cover" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <p className="text-sm text-white truncate">{file.name}</p>
                          <p className="text-xs text-zinc-400">Level {file.level} {file.animated ? '• Animated' : ''}</p>
                        </div>
                        <div className="flex items-center gap-2">
                          {file.status === 'pending' && <Clock className="h-4 w-4 text-zinc-400" />}
                          {file.status === 'processing' && <RefreshCw className="h-4 w-4 text-purple-400 animate-spin" />}
                          {file.status === 'success' && <Check className="h-4 w-4 text-green-400" />}
                          {file.status === 'error' && <AlertCircle className="h-4 w-4 text-red-400" />}
                        </div>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Navigation */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.6 }}
        >
          <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
            <CardContent className="p-6">
              <div className="flex items-center justify-between">
                <Button
                  variant="outline"
                  onClick={prevStep}
                  disabled={currentStep === 1}
                  className="border-zinc-700/50 hover:bg-zinc-800/50"
                >
                  <ArrowLeft className="mr-2 h-4 w-4" />
                  Previous
                </Button>

                <div className="flex items-center gap-2">
                  {steps.map((step) => (
                    <div
                      key={step.number}
                      className={`w-2 h-2 rounded-full transition-all ${
                        currentStep >= step.number
                          ? 'bg-gradient-to-r from-purple-500 to-cyan-500'
                          : 'bg-zinc-600'
                      }`}
                    />
                  ))}
                </div>

                {currentStep < 3 ? (
                  <Button
                    onClick={nextStep}
                    disabled={
                      (currentStep === 1 && !canProceedToStep2) ||
                      (currentStep === 2 && !canProceedToStep3)
                    }
                    className="bg-gradient-to-r from-purple-600 to-cyan-600 hover:from-purple-700 hover:to-cyan-700 text-white"
                  >
                    Next
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </Button>
                ) : (
                  <Button
                    onClick={handleImport}
                    disabled={isImporting || importedFiles.length === 0}
                    className="bg-gradient-to-r from-green-600 to-emerald-600 hover:from-green-700 hover:to-emerald-700 text-white"
                  >
                    {isImporting ? (
                      <>
                        <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                        Importing...
                      </>
                    ) : (
                      <>
                        <Upload className="mr-2 h-4 w-4" />
                        Start Import
                      </>
                    )}
                  </Button>
                )}
              </div>
            </CardContent>
          </Card>
        </motion.div>
      </div>
    </div>
  );
}