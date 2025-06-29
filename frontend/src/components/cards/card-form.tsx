"use client";

import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Upload, X, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { CardDTO, CollectionDTO } from "@/lib/types";
import { apiClient } from "@/lib/api";
import { validateCardData, validateImageFile } from "@/lib/validations";
import * as z from "zod";

// Define the form schema directly here to avoid type conflicts
const cardFormSchema = z.object({
  name: z.string().min(1, "Card name is required").max(100, "Card name must be less than 100 characters"),
  collection_id: z.string().min(1, "Collection is required"),
  level: z.number().int().min(1).max(5),
  animated: z.boolean(),
  promo: z.boolean(),
  tags: z.array(z.string()),
  image_file: z.instanceof(File).optional(),
});

type CardFormValues = z.infer<typeof cardFormSchema>;

interface CardFormProps {
  card?: CardDTO;
  collections: CollectionDTO[];
  onSuccess?: () => void;
}

export function CardForm({ card, collections, onSuccess }: CardFormProps) {
  const router = useRouter();
  const [isLoading, setIsLoading] = React.useState(false);
  const [uploadProgress, setUploadProgress] = React.useState(0);
  const [imagePreview, setImagePreview] = React.useState<string | null>(
    card?.image_url || null
  );
  const [selectedImage, setSelectedImage] = React.useState<File | null>(null);
  const [tagInput, setTagInput] = React.useState("");
  const fileInputRef = React.useRef<HTMLInputElement>(null);

  const form = useForm<CardFormValues>({
    resolver: zodResolver(cardFormSchema),
    defaultValues: {
      name: card?.name || "",
      collection_id: card?.collection_id || "",
      level: card?.level || 1,
      animated: card?.animated ?? false,
      promo: card?.promo ?? false,
      tags: card?.tags || [],
    },
  });

  const watchedTags = form.watch("tags");

  const handleImageSelect = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      // Validate file using enhanced Zod validation
      if (!validateImageFile(file)) {
        toast.error("Please select a valid image file (JPEG, PNG, GIF, or WebP) under 10MB");
        return;
      }

      setSelectedImage(file);

      // Create preview
      const reader = new FileReader();
      reader.onload = (e) => {
        setImagePreview(e.target?.result as string);
      };
      reader.readAsDataURL(file);
    }
  };

  const removeImage = () => {
    setSelectedImage(null);
    setImagePreview(card?.image_url || null);
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const addTag = (tag: string) => {
    const trimmedTag = tag.trim();
    if (trimmedTag && !watchedTags.includes(trimmedTag)) {
      form.setValue("tags", [...watchedTags, trimmedTag]);
      setTagInput("");
    }
  };

  const removeTag = (tagToRemove: string) => {
    form.setValue("tags", watchedTags.filter(tag => tag !== tagToRemove));
  };

  const handleTagKeyPress = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter" || event.key === ",") {
      event.preventDefault();
      addTag(tagInput);
    }
  };

  const onSubmit = async (values: CardFormValues) => {
    setIsLoading(true);
    setUploadProgress(0);

    try {
      // Validate data using enhanced Zod validation
      const validatedData = validateCardData(values);
      
      // Create FormData for file upload
      const formData = new FormData();
      formData.append("name", validatedData.name);
      formData.append("collection_id", validatedData.collection_id);
      formData.append("level", validatedData.level.toString());
      formData.append("animated", validatedData.animated.toString());
      formData.append("promo", validatedData.promo.toString());
      formData.append("tags", JSON.stringify(validatedData.tags));

      if (selectedImage) {
        // Additional validation for the selected image
        if (!validateImageFile(selectedImage)) {
          throw new Error("Invalid image file selected");
        }
        formData.append("image", selectedImage);
      }

      let result: CardDTO;
      if (card) {
        // Update existing card
        result = await apiClient.updateCard(card.id, formData);
        toast.success("Card updated successfully");
      } else {
        // Create new card
        result = await apiClient.createCard(formData);
        toast.success("Card created successfully");
      }

      // Reset form or redirect
      if (onSuccess) {
        onSuccess();
      } else {
        router.push("/dashboard/cards");
      }
    } catch (error: any) {
      console.error("Failed to save card:", error);
      
      // Enhanced error handling
      if (error.name === 'ZodError') {
        toast.error("Please check your input data and try again.");
      } else {
        toast.error(error.message || "Failed to save card. Please try again.");
      }
    } finally {
      setIsLoading(false);
      setUploadProgress(0);
    }
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <Card className="bg-zinc-900 border-zinc-800">
        <CardHeader>
          <CardTitle className="text-white">
            {card ? "Edit Card" : "Create New Card"}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
              {/* Image Upload */}
              <div className="space-y-4">
                <label className="text-sm font-medium text-zinc-300">Card Image</label>
                
                {imagePreview ? (
                  <div className="relative w-48 h-64 mx-auto">
                    <img
                      src={imagePreview}
                      alt="Card preview"
                      className="w-full h-full object-cover rounded-lg border border-zinc-700"
                    />
                    <Button
                      type="button"
                      variant="destructive"
                      size="sm"
                      onClick={removeImage}
                      className="absolute top-2 right-2"
                    >
                      <X className="h-4 w-4" />
                    </Button>
                  </div>
                ) : (
                  <div
                    onClick={() => fileInputRef.current?.click()}
                    className="w-48 h-64 mx-auto border-2 border-dashed border-zinc-700 rounded-lg flex flex-col items-center justify-center cursor-pointer hover:border-zinc-600 transition-colors"
                  >
                    <Upload className="h-8 w-8 text-zinc-400 mb-2" />
                    <p className="text-sm text-zinc-400 text-center">
                      Click to upload an image
                    </p>
                    <p className="text-xs text-zinc-500 text-center mt-1">
                      PNG, JPG, GIF up to 10MB
                    </p>
                  </div>
                )}

                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*"
                  onChange={handleImageSelect}
                  className="hidden"
                />

                {uploadProgress > 0 && uploadProgress < 100 && (
                  <div className="space-y-2">
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-zinc-400">Uploading...</span>
                      <span className="text-zinc-400">{uploadProgress}%</span>
                    </div>
                    <Progress value={uploadProgress} className="w-full" />
                  </div>
                )}
              </div>

              {/* Card Name */}
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className="text-zinc-300">Card Name</FormLabel>
                    <FormControl>
                      <Input
                        placeholder="Enter card name..."
                        {...field}
                        className="bg-zinc-800 border-zinc-700 text-white"
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Collection */}
              <FormField
                control={form.control}
                name="collection_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className="text-zinc-300">Collection</FormLabel>
                    <Select onValueChange={field.onChange} defaultValue={field.value}>
                      <FormControl>
                        <SelectTrigger className="bg-zinc-800 border-zinc-700 text-white">
                          <SelectValue placeholder="Select a collection" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent className="bg-zinc-900 border-zinc-800">
                        {collections.map((collection) => (
                          <SelectItem
                            key={collection.id}
                            value={collection.id}
                            className="text-zinc-300 hover:bg-zinc-800"
                          >
                            {collection.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Level */}
              <FormField
                control={form.control}
                name="level"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className="text-zinc-300">Level</FormLabel>
                    <Select onValueChange={(value) => field.onChange(parseInt(value))} value={field.value?.toString()}>
                      <FormControl>
                        <SelectTrigger className="bg-zinc-800 border-zinc-700 text-white">
                          <SelectValue placeholder="Select level" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent className="bg-zinc-900 border-zinc-800">
                        {[1, 2, 3, 4, 5].map((level) => (
                          <SelectItem
                            key={level}
                            value={level.toString()}
                            className="text-zinc-300 hover:bg-zinc-800"
                          >
                            Level {level}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Card Type Checkboxes */}
              <div className="space-y-4">
                <label className="text-sm font-medium text-zinc-300">Card Type</label>
                <div className="space-y-3">
                  <FormField
                    control={form.control}
                    name="animated"
                    render={({ field }) => (
                      <FormItem className="flex items-center space-x-2">
                        <FormControl>
                          <input
                            type="checkbox"
                            checked={field.value}
                            onChange={field.onChange}
                            className="rounded border-zinc-600 bg-zinc-800 text-pink-600 focus:ring-pink-600"
                          />
                        </FormControl>
                        <FormLabel className="text-zinc-300 !mt-0">
                          Animated Card
                        </FormLabel>
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name="promo"
                    render={({ field }) => (
                      <FormItem className="flex items-center space-x-2">
                        <FormControl>
                          <input
                            type="checkbox"
                            checked={field.value}
                            onChange={field.onChange}
                            className="rounded border-zinc-600 bg-zinc-800 text-pink-600 focus:ring-pink-600"
                          />
                        </FormControl>
                        <FormLabel className="text-zinc-300 !mt-0">
                          Promotional Card
                        </FormLabel>
                      </FormItem>
                    )}
                  />
                </div>
              </div>

              {/* Tags */}
              <div className="space-y-4">
                <label className="text-sm font-medium text-zinc-300">Tags</label>
                
                {/* Tag Input */}
                <div className="flex gap-2">
                  <Input
                    placeholder="Add a tag..."
                    value={tagInput}
                    onChange={(e) => setTagInput(e.target.value)}
                    onKeyDown={handleTagKeyPress}
                    className="bg-zinc-800 border-zinc-700 text-white"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => addTag(tagInput)}
                    className="border-zinc-700 hover:bg-zinc-800"
                  >
                    Add
                  </Button>
                </div>

                {/* Tags Display */}
                {watchedTags.length > 0 && (
                  <div className="flex flex-wrap gap-2">
                    {watchedTags.map((tag, index) => (
                      <Badge
                        key={index}
                        variant="secondary"
                        className="bg-zinc-700 text-zinc-300 hover:bg-zinc-600"
                      >
                        {tag}
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={() => removeTag(tag)}
                          className="ml-1 h-4 w-4 p-0 hover:bg-zinc-600"
                        >
                          <X className="h-3 w-3" />
                        </Button>
                      </Badge>
                    ))}
                  </div>
                )}
              </div>

              {/* Submit Buttons */}
              <div className="flex gap-3 pt-6">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => router.back()}
                  className="border-zinc-700 hover:bg-zinc-800"
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  disabled={isLoading}
                  className="bg-pink-600 hover:bg-pink-700"
                >
                  {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  {card ? "Update Card" : "Create Card"}
                </Button>
              </div>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}