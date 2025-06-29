import { notFound } from "next/navigation";
import { CardForm } from "@/components/cards/card-form";
import { CardDTO, CollectionDTO } from "@/lib/types";

interface PageProps {
  params: Promise<{
    id: string;
  }>;
}

// Fetch card and collections data
async function getCardData(id: string): Promise<{
  card: CardDTO | null;
  collections: CollectionDTO[];
}> {
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';
  
  try {
    const [cardResponse, collectionsResponse] = await Promise.all([
      fetch(`${backendUrl}/admin/cards/${id}`, {
        headers: { 'Accept': 'application/json' },
        credentials: 'include',
      }),
      fetch(`${backendUrl}/admin/api/collections`, {
        headers: { 'Accept': 'application/json' },
        credentials: 'include',
      })
    ]);
    
    const collections = collectionsResponse.ok 
      ? (await collectionsResponse.json()).data || []
      : [];

    if (!cardResponse.ok) {
      return { card: null, collections };
    }
    
    const cardResult = await cardResponse.json();
    return { 
      card: cardResult.data || null, 
      collections 
    };
  } catch (error) {
    console.error('Failed to fetch card data:', error);
    return { card: null, collections: [] };
  }
}

export default async function EditCardPage({ params }: PageProps) {
  const { id } = await params;
  const { card, collections } = await getCardData(id);

  if (!card) {
    notFound();
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-white">Edit Card</h1>
        <p className="text-zinc-400">Modify card details and settings</p>
      </div>

      {/* Form */}
      <CardForm card={card} collections={collections} />
    </div>
  );
}