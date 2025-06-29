"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { toast } from "sonner";

// UI Components
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";

// Icons
import { Plus, Loader2, FolderOpen } from "lucide-react";

// Types
import type { CollectionDTO } from "@/lib/types";

const collectionSchema = z.object({
  name: z.string().min(1, "Collection name is required").max(100, "Name too long"),
  description: z.string().default(""),
  collection_type: z.enum(["girl_group", "boy_group", "other"], {
    required_error: "Please select a collection type",
  }),
  promo: z.boolean().default(false),
});

type CollectionFormData = z.infer<typeof collectionSchema>;

interface CollectionFormProps {
  collection?: CollectionDTO;
}

export function CollectionForm({ collection }: CollectionFormProps) {
  const router = useRouter();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const isEditing = !!collection;

  const form = useForm<CollectionFormData>({
    resolver: zodResolver(collectionSchema),
    defaultValues: {
      name: collection?.name || "",
      description: collection?.description || "",
      collection_type: collection?.collection_type || "girl_group",
      promo: collection?.promo || false,
    },
  });

  const onSubmit = async (data: CollectionFormData) => {
    setIsSubmitting(true);
    
    try {
      const url = isEditing 
        ? `/api/collections/${collection.id}`
        : '/api/collections';
      
      const method = isEditing ? 'PUT' : 'POST';
      
      const response = await fetch(url, {
        method,
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({ message: 'Unknown error' }));
        throw new Error(error.message || `Failed to ${isEditing ? 'update' : 'create'} collection`);
      }

      const result = await response.json();
      
      if (!result.success) {
        throw new Error(result.error || `Failed to ${isEditing ? 'update' : 'create'} collection`);
      }

      toast.success(`Collection ${isEditing ? 'updated' : 'created'} successfully!`);
      router.push('/dashboard/collections');
      router.refresh();
    } catch (error: any) {
      console.error(`Failed to ${isEditing ? 'update' : 'create'} collection:`, error);
      toast.error(error.message || `Failed to ${isEditing ? 'update' : 'create'} collection`);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto">
      <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
        <CardHeader>
          <CardTitle className="text-2xl text-white flex items-center gap-3">
            <FolderOpen className="h-6 w-6 text-blue-400" />
            {isEditing ? 'Edit Collection' : 'Create New Collection'}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
              {/* Collection Name */}
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className="text-white">Collection Name</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder="Enter collection name (e.g., BLACKPINK, BTS, TWICE)"
                        className="bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white focus:border-blue-500/50 focus:ring-blue-500/20"
                      />
                    </FormControl>
                    <FormDescription className="text-zinc-400">
                      A unique name for your collection
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Collection Type */}
              <FormField
                control={form.control}
                name="collection_type"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className="text-white">Collection Type</FormLabel>
                    <Select onValueChange={field.onChange} defaultValue={field.value}>
                      <FormControl>
                        <SelectTrigger className="bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                          <SelectValue placeholder="Select collection type" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                        <SelectItem value="girl_group">Girl Group</SelectItem>
                        <SelectItem value="boy_group">Boy Group</SelectItem>
                        <SelectItem value="other">Other</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormDescription className="text-zinc-400">
                      Categorize your collection
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Description */}
              <FormField
                control={form.control}
                name="description"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className="text-white">Description (Optional)</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        placeholder="Describe your collection..."
                        className="bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white focus:border-blue-500/50 focus:ring-blue-500/20 min-h-[100px]"
                      />
                    </FormControl>
                    <FormDescription className="text-zinc-400">
                      Add details about this collection
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Promo Collection Toggle */}
              <FormField
                control={form.control}
                name="promo"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-center justify-between rounded-lg border border-zinc-700/50 p-4 bg-black/20">
                    <div className="space-y-0.5">
                      <FormLabel className="text-white font-medium">
                        Promotional Collection
                      </FormLabel>
                      <FormDescription className="text-zinc-400">
                        Mark this as a special promotional collection
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              {/* Actions */}
              <div className="flex gap-4 pt-6">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => router.back()}
                  className="flex-1 border-zinc-700/50 hover:bg-zinc-800/50"
                  disabled={isSubmitting}
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  disabled={isSubmitting}
                  className="flex-1 bg-gradient-to-r from-blue-600 via-blue-500 to-purple-600 hover:from-blue-700 hover:via-blue-600 hover:to-purple-700 text-white"
                >
                  {isSubmitting ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {isEditing ? 'Updating...' : 'Creating...'}
                    </>
                  ) : (
                    <>
                      <Plus className="mr-2 h-4 w-4" />
                      {isEditing ? 'Update Collection' : 'Create Collection'}
                    </>
                  )}
                </Button>
              </div>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}