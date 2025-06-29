"use client";

import React, { useCallback, useState } from "react";
import { useDropzone } from "react-dropzone";
import { Upload, X, FileImage, AlertCircle, CheckCircle } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { toast } from "sonner";

export interface FileWithPreview extends File {
  preview?: string;
}

export interface UploadedFile {
  file: FileWithPreview;
  progress: number;
  status: "pending" | "uploading" | "success" | "error";
  error?: string;
  url?: string;
}

interface FileUploadProps {
  value?: FileWithPreview[];
  onChange?: (files: FileWithPreview[]) => void;
  onUpload?: (files: FileWithPreview[]) => Promise<string[]>;
  accept?: Record<string, string[]>;
  maxFiles?: number;
  maxSize?: number;
  disabled?: boolean;
  className?: string;
}

export function FileUpload({
  value = [],
  onChange,
  onUpload,
  accept = {
    "image/*": [".png", ".jpg", ".jpeg", ".gif", ".webp"]
  },
  maxFiles = 10,
  maxSize = 5 * 1024 * 1024, // 5MB
  disabled = false,
  className,
}: FileUploadProps) {
  const [uploadedFiles, setUploadedFiles] = useState<UploadedFile[]>([]);
  const [isUploading, setIsUploading] = useState(false);

  const onDrop = useCallback(
    (acceptedFiles: File[], rejectedFiles: any[]) => {
      if (rejectedFiles.length > 0) {
        rejectedFiles.forEach((rejection) => {
          const error = rejection.errors[0];
          toast.error(`File ${rejection.file.name}: ${error.message}`);
        });
      }

      if (acceptedFiles.length > 0) {
        const filesWithPreview = acceptedFiles.map((file) => {
          const fileWithPreview = Object.assign(file, {
            preview: URL.createObjectURL(file),
          });
          return fileWithPreview;
        });

        const newFiles = [...value, ...filesWithPreview].slice(0, maxFiles);
        onChange?.(newFiles);

        // Initialize upload states
        const newUploadedFiles = filesWithPreview.map((file) => ({
          file,
          progress: 0,
          status: "pending" as const,
        }));
        
        setUploadedFiles((prev) => [...prev, ...newUploadedFiles]);
      }
    },
    [value, onChange, maxFiles]
  );

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    accept,
    maxFiles: maxFiles - value.length,
    maxSize,
    disabled: disabled || isUploading,
  });

  const removeFile = useCallback(
    (index: number) => {
      const newFiles = value.filter((_, i) => i !== index);
      onChange?.(newFiles);
      
      // Clean up object URL
      if (value[index]?.preview) {
        URL.revokeObjectURL(value[index].preview!);
      }
      
      // Remove from uploaded files state
      setUploadedFiles((prev) => prev.filter((_, i) => i !== index));
    },
    [value, onChange]
  );

  const uploadFiles = async () => {
    if (!onUpload || value.length === 0) return;

    setIsUploading(true);
    
    try {
      // Simulate upload progress
      for (let i = 0; i < value.length; i++) {
        setUploadedFiles((prev) =>
          prev.map((item, index) =>
            index === i ? { ...item, status: "uploading" } : item
          )
        );

        // Simulate progress updates
        for (let progress = 0; progress <= 100; progress += 10) {
          await new Promise((resolve) => setTimeout(resolve, 100));
          setUploadedFiles((prev) =>
            prev.map((item, index) =>
              index === i ? { ...item, progress } : item
            )
          );
        }

        setUploadedFiles((prev) =>
          prev.map((item, index) =>
            index === i ? { ...item, status: "success" } : item
          )
        );
      }

      const urls = await onUpload(value);
      
      setUploadedFiles((prev) =>
        prev.map((item, index) => ({
          ...item,
          url: urls[index],
          status: "success",
        }))
      );

      toast.success(`Successfully uploaded ${value.length} file(s)`);
    } catch (error) {
      console.error("Upload failed:", error);
      setUploadedFiles((prev) =>
        prev.map((item) => ({
          ...item,
          status: "error",
          error: "Upload failed",
        }))
      );
      toast.error("Upload failed. Please try again.");
    } finally {
      setIsUploading(false);
    }
  };

  const getStatusIcon = (status: UploadedFile["status"]) => {
    switch (status) {
      case "success":
        return <CheckCircle className="h-4 w-4 text-green-400" />;
      case "error":
        return <AlertCircle className="h-4 w-4 text-red-400" />;
      default:
        return <FileImage className="h-4 w-4 text-zinc-400" />;
    }
  };

  return (
    <div className={cn("space-y-4", className)}>
      {/* Dropzone */}
      <div
        {...getRootProps()}
        className={cn(
          "relative rounded-lg border-2 border-dashed border-zinc-700 bg-zinc-900/50 p-6 text-center transition-colors hover:bg-zinc-900/80",
          isDragActive && "border-blue-500 bg-blue-500/10",
          disabled && "cursor-not-allowed opacity-50",
          !disabled && "cursor-pointer"
        )}
      >
        <input {...getInputProps()} />
        
        <div className="flex flex-col items-center justify-center space-y-3">
          <Upload className={cn(
            "h-8 w-8",
            isDragActive ? "text-blue-400" : "text-zinc-400"
          )} />
          
          {isDragActive ? (
            <p className="text-blue-400">Drop files here...</p>
          ) : (
            <div className="space-y-1">
              <p className="text-zinc-300">
                <span className="font-medium">Click to upload</span> or drag and drop
              </p>
              <p className="text-sm text-zinc-500">
                PNG, JPG, GIF up to {Math.round(maxSize / 1024 / 1024)}MB each
              </p>
            </div>
          )}
        </div>
      </div>

      {/* File List */}
      {value.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-sm font-medium text-zinc-300">
              Selected Files ({value.length}/{maxFiles})
            </p>
            {value.length > 0 && onUpload && (
              <Button
                onClick={uploadFiles}
                disabled={isUploading}
                size="sm"
                className="bg-blue-600 hover:bg-blue-700"
              >
                {isUploading ? "Uploading..." : "Upload Files"}
              </Button>
            )}
          </div>

          <div className="space-y-2">
            {value.map((file, index) => {
              const uploadedFile = uploadedFiles[index];
              
              return (
                <div
                  key={`${file.name}-${index}`}
                  className="flex items-center space-x-3 rounded-lg border border-zinc-800 bg-zinc-900 p-3"
                >
                  {/* File Preview */}
                  <div className="flex-shrink-0">
                    {file.preview ? (
                      <img
                        src={file.preview}
                        alt={file.name}
                        className="h-10 w-10 rounded object-cover"
                        onLoad={() => URL.revokeObjectURL(file.preview!)}
                      />
                    ) : (
                      <div className="flex h-10 w-10 items-center justify-center rounded bg-zinc-800">
                        <FileImage className="h-5 w-5 text-zinc-400" />
                      </div>
                    )}
                  </div>

                  {/* File Info */}
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center space-x-2">
                      {uploadedFile && getStatusIcon(uploadedFile.status)}
                      <p className="truncate text-sm font-medium text-zinc-300">
                        {file.name}
                      </p>
                    </div>
                    <p className="text-xs text-zinc-500">
                      {(file.size / 1024 / 1024).toFixed(2)} MB
                    </p>
                    
                    {/* Progress Bar */}
                    {uploadedFile && uploadedFile.status === "uploading" && (
                      <Progress 
                        value={uploadedFile.progress} 
                        className="mt-1 h-1"
                      />
                    )}
                    
                    {/* Error Message */}
                    {uploadedFile && uploadedFile.status === "error" && uploadedFile.error && (
                      <p className="text-xs text-red-400 mt-1">
                        {uploadedFile.error}
                      </p>
                    )}
                  </div>

                  {/* Remove Button */}
                  <Button
                    onClick={() => removeFile(index)}
                    variant="ghost"
                    size="sm"
                    className="flex-shrink-0 text-zinc-400 hover:text-red-400"
                    disabled={isUploading}
                  >
                    <X className="h-4 w-4" />
                  </Button>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}