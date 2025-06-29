import * as z from "zod";

// Enhanced validation schemas with custom error messages and refined rules

// Common validators
const requiredString = (field: string) => 
  z.string().min(1, `${field} is required`).trim();

const optionalString = z.string().optional().or(z.literal(""));

const fileSchema = z.object({
  name: z.string(),
  size: z.number(),
  type: z.string(),
  lastModified: z.number(),
});

// User validation schemas
export const userLoginSchema = z.object({
  discord_id: requiredString("Discord ID"),
  username: z.string().min(2, "Username must be at least 2 characters").max(50, "Username must be less than 50 characters"),
});

export const userProfileSchema = z.object({
  username: z.string()
    .min(2, "Username must be at least 2 characters")
    .max(50, "Username must be less than 50 characters")
    .regex(/^[a-zA-Z0-9_-]+$/, "Username can only contain letters, numbers, underscores, and hyphens"),
  avatar_url: z.string().url("Invalid avatar URL").optional().or(z.literal("")),
  bio: z.string().max(500, "Bio must be less than 500 characters").optional(),
});

// Card validation schemas
export const cardSchema = z.object({
  id: z.string().uuid("Invalid card ID").optional(),
  name: requiredString("Card name")
    .min(1, "Card name is required")
    .max(100, "Card name must be less than 100 characters")
    .regex(/^[^<>\"'&]*$/, "Card name contains invalid characters"),
  collection_id: requiredString("Collection")
    .uuid("Invalid collection ID"),
  level: z.number()
    .int("Level must be a whole number")
    .min(1, "Level must be at least 1")
    .max(5, "Level must be at most 5"),
  animated: z.boolean().optional().default(false),
  promo: z.boolean().optional().default(false),
  tags: z.array(z.string()
    .min(1, "Tag cannot be empty")
    .max(30, "Tag must be less than 30 characters")
    .regex(/^[a-zA-Z0-9\s-_]+$/, "Tag contains invalid characters"))
    .max(10, "Maximum 10 tags allowed")
    .optional()
    .default([]),
  image_url: z.string().url("Invalid image URL").optional(),
  image_file: z.instanceof(File, { message: "Invalid image file" }).optional(),
});

export const cardFormSchema = cardSchema.omit({ id: true, image_url: true });

export const cardUpdateSchema = cardSchema.partial().required({ 
  id: true 
});

export const bulkCardActionSchema = z.object({
  action: z.enum(["delete", "update_level", "update_collection", "add_tags", "remove_tags"], {
    errorMap: () => ({ message: "Invalid bulk action" })
  }),
  card_ids: z.array(z.string().uuid("Invalid card ID"))
    .min(1, "At least one card must be selected")
    .max(100, "Cannot perform bulk action on more than 100 cards"),
  data: z.record(z.any()).optional(),
});

// Collection validation schemas
export const collectionSchema = z.object({
  id: z.string().uuid("Invalid collection ID").optional(),
  name: requiredString("Collection name")
    .min(1, "Collection name is required")
    .max(100, "Collection name must be less than 100 characters")
    .regex(/^[^<>\"'&]*$/, "Collection name contains invalid characters"),
  description: z.string()
    .max(1000, "Description must be less than 1000 characters")
    .optional()
    .or(z.literal("")),
  collection_type: z.enum(["girl_group", "boy_group", "other"], {
    errorMap: () => ({ message: "Invalid collection type" })
  }),
  promo: z.boolean().default(false),
  image_url: z.string().url("Invalid image URL").optional(),
  card_count: z.number().int().min(0).optional(),
  created_at: z.string().datetime().optional(),
  updated_at: z.string().datetime().optional(),
});

export const collectionFormSchema = collectionSchema.omit({ 
  id: true, 
  card_count: true, 
  created_at: true, 
  updated_at: true 
});

// File upload validation schemas
export const fileUploadSchema = z.object({
  files: z.array(fileSchema).min(1, "At least one file must be uploaded"),
  max_size: z.number().default(10 * 1024 * 1024), // 10MB
  allowed_types: z.array(z.string()).default(["image/jpeg", "image/png", "image/gif", "image/webp"]),
});

export const imageFileSchema = z.object({
  file: z.instanceof(File)
    .refine((file) => file.size <= 10 * 1024 * 1024, "File size must be less than 10MB")
    .refine((file) => ["image/jpeg", "image/png", "image/gif", "image/webp"].includes(file.type), 
      "File must be an image (JPEG, PNG, GIF, or WebP)"),
  preview: z.string().url().optional(),
});

// Import wizard validation schemas
export const importWizardStep1Schema = z.object({
  mode: z.enum(["new_collection", "existing_collection"], {
    errorMap: () => ({ message: "Please select import mode" })
  }),
  collection_id: z.string().uuid("Invalid collection ID").optional(),
  collection_info: collectionFormSchema.optional(),
}).refine((data) => {
  if (data.mode === "existing_collection") {
    return !!data.collection_id;
  }
  if (data.mode === "new_collection") {
    return !!data.collection_info;
  }
  return false;
}, {
  message: "Collection information is required",
  path: ["collection_info"]
});

export const importWizardStep2Schema = z.object({
  files: z.array(imageFileSchema).min(1, "At least one image file must be uploaded"),
  naming_pattern: z.enum(["auto", "manual", "filename"]).default("auto"),
  auto_level_detection: z.boolean().default(true),
  auto_animated_detection: z.boolean().default(true),
});

export const importCardSchema = z.object({
  name: requiredString("Card name").max(100, "Card name must be less than 100 characters"),
  level: z.number().int().min(1).max(5),
  animated: z.boolean().default(false),
  tags: z.array(z.string().max(30)).max(10).default([]),
  file: z.instanceof(File),
  preview: z.string().url().optional(),
});

export const importWizardStep3Schema = z.object({
  cards: z.array(importCardSchema).min(1, "At least one card must be configured"),
  apply_to_all: z.object({
    level: z.number().int().min(1).max(5).optional(),
    animated: z.boolean().optional(),
    tags: z.array(z.string()).optional(),
  }).optional(),
});

export const importWizardFinalSchema = z.object({
  step1: importWizardStep1Schema,
  step2: importWizardStep2Schema,
  step3: importWizardStep3Schema,
  create_collection: z.boolean().default(false),
  import_settings: z.object({
    skip_duplicates: z.boolean().default(true),
    overwrite_existing: z.boolean().default(false),
    create_backups: z.boolean().default(true),
  }).default({}),
});

// Search and filtering schemas
export const cardSearchSchema = z.object({
  search: optionalString,
  collection: optionalString,
  level: z.number().int().min(1).max(5).optional(),
  animated: z.boolean().optional(),
  promo: z.boolean().optional(),
  tags: z.array(z.string()).optional(),
  page: z.number().int().min(1).default(1),
  limit: z.number().int().min(1).max(100).default(50),
  sort_by: z.enum(["name", "level", "created_at", "updated_at"]).default("created_at"),
  sort_order: z.enum(["asc", "desc"]).default("desc"),
});

export const collectionSearchSchema = z.object({
  search: optionalString,
  collection_type: z.enum(["girl_group", "boy_group", "other"]).optional(),
  promo: z.boolean().optional(),
  page: z.number().int().min(1).default(1),
  limit: z.number().int().min(1).max(100).default(20),
  sort_by: z.enum(["name", "created_at", "updated_at", "card_count"]).default("created_at"),
  sort_order: z.enum(["asc", "desc"]).default("desc"),
});

// API response schemas
export const apiResponseSchema = z.object({
  success: z.boolean(),
  message: z.string().optional(),
  data: z.any().optional(),
  error: z.string().optional(),
  errors: z.record(z.string()).optional(),
});

export const paginationSchema = z.object({
  total: z.number().int().min(0),
  page: z.number().int().min(1),
  limit: z.number().int().min(1),
  total_pages: z.number().int().min(0),
  has_more: z.boolean(),
  has_prev: z.boolean(),
});

export const paginatedResponseSchema = <T extends z.ZodType>(dataSchema: T) =>
  z.object({
    data: z.array(dataSchema),
    pagination: paginationSchema,
  });

// Type exports
export type CardFormValues = z.infer<typeof cardFormSchema>;
export type CardUpdateValues = z.infer<typeof cardUpdateSchema>;
export type CollectionFormValues = z.infer<typeof collectionFormSchema>;
export type ImportWizardStep1Values = z.infer<typeof importWizardStep1Schema>;
export type ImportWizardStep2Values = z.infer<typeof importWizardStep2Schema>;
export type ImportWizardStep3Values = z.infer<typeof importWizardStep3Schema>;
export type ImportCardValues = z.infer<typeof importCardSchema>;
export type CardSearchValues = z.infer<typeof cardSearchSchema>;
export type CollectionSearchValues = z.infer<typeof collectionSearchSchema>;
export type BulkCardActionValues = z.infer<typeof bulkCardActionSchema>;
export type ApiResponse<T = any> = z.infer<typeof apiResponseSchema> & { data?: T };
export type PaginatedResponse<T> = z.infer<ReturnType<typeof paginatedResponseSchema<z.ZodType<T>>>>;

// Validation helper functions
export function validateCardData(data: unknown): CardFormValues {
  return cardFormSchema.parse(data);
}

export function validateCollectionData(data: unknown): CollectionFormValues {
  return collectionFormSchema.parse(data);
}

export function validateSearchParams(data: unknown): CardSearchValues {
  return cardSearchSchema.parse(data);
}

export function validateImageFile(file: File): boolean {
  try {
    imageFileSchema.parse({ file });
    return true;
  } catch {
    return false;
  }
}

// Custom validation rules
export const customValidators = {
  isValidImageFile: (file: File) => {
    return file.type.startsWith("image/") && file.size <= 10 * 1024 * 1024;
  },
  
  isValidCardName: (name: string) => {
    return name.length >= 1 && name.length <= 100 && !/[<>"'&]/.test(name);
  },
  
  isValidLevel: (level: number) => {
    return Number.isInteger(level) && level >= 1 && level <= 5;
  },
  
  isValidCollectionType: (type: string) => {
    return ["girl_group", "boy_group", "other"].includes(type);
  },
};