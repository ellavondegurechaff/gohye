import { CollectionForm } from "@/components/collections/collection-form";

export default function NewCollectionPage() {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-white">Create New Collection</h1>
        <p className="text-zinc-400">Add a new K-pop trading card collection</p>
      </div>

      {/* Form */}
      <CollectionForm />
    </div>
  );
}