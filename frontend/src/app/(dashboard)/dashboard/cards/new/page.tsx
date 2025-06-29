import { CardForm } from "@/components/cards/card-form";
import { CollectionDTO } from "@/lib/types";

// Fetch collections for the form
async function getCollections(): Promise<CollectionDTO[]> {
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';
  
  try {
    const response = await fetch(`${backendUrl}/admin/api/collections`, {
      headers: { 'Accept': 'application/json' },
      credentials: 'include',
    });
    
    if (!response.ok) {
      console.error('Failed to fetch collections:', response.status);
      return [];
    }
    
    const result = await response.json();
    return result.data || [];
  } catch (error) {
    console.error('Failed to fetch collections:', error);
    return [];
  }
}

export default async function NewCardPage() {
  const collections = await getCollections();

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-white">Add New Card</h1>
        <p className="text-zinc-400">Create a new card for your collection</p>
      </div>

      {/* Form */}
      <CardForm collections={collections} />
    </div>
  );
}