import { notFound } from "next/navigation";
import Link from "next/link";
import Image from "next/image";
import { Edit, ArrowLeft, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { CardDTO } from "@/lib/types";

interface PageProps {
  params: Promise<{
    id: string;
  }>;
}

// Fetch card data
async function getCard(id: string): Promise<CardDTO | null> {
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';
  
  try {
    const response = await fetch(`${backendUrl}/admin/cards/${id}`, {
      headers: { 'Accept': 'application/json' },
      credentials: 'include',
    });
    
    if (!response.ok) {
      return null;
    }
    
    const result = await response.json();
    return result.data || null;
  } catch (error) {
    console.error('Failed to fetch card:', error);
    return null;
  }
}

export default async function CardDetailPage({ params }: PageProps) {
  const { id } = await params;
  const card = await getCard(id);

  if (!card) {
    notFound();
  }

  const getLevelVariant = (level: number) => {
    switch (level) {
      case 5:
        return "bg-yellow-500/20 text-yellow-400 border-yellow-500/50";
      case 4:
        return "bg-purple-500/20 text-purple-400 border-purple-500/50";
      case 3:
        return "bg-blue-500/20 text-blue-400 border-blue-500/50";
      case 2:
        return "bg-green-500/20 text-green-400 border-green-500/50";
      default:
        return "bg-gray-500/20 text-gray-400 border-gray-500/50";
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button asChild variant="outline" className="border-zinc-700 hover:bg-zinc-800">
            <Link href="/dashboard/cards">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to Cards
            </Link>
          </Button>
          <div>
            <h1 className="text-3xl font-bold text-white">{card.name}</h1>
            <p className="text-zinc-400">Card Details</p>
          </div>
        </div>

        <div className="flex gap-2">
          <Button asChild variant="outline" className="border-zinc-700 hover:bg-zinc-800">
            <Link href={`/dashboard/cards/${card.id}/edit`}>
              <Edit className="mr-2 h-4 w-4" />
              Edit Card
            </Link>
          </Button>
          <Button variant="destructive">
            <Trash2 className="mr-2 h-4 w-4" />
            Delete Card
          </Button>
        </div>
      </div>

      {/* Card Content */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Card Image */}
        <div className="lg:col-span-1">
          <Card className="bg-zinc-900 border-zinc-800">
            <CardContent className="p-6">
              <div className="aspect-[3/4] bg-zinc-800 rounded-lg overflow-hidden">
                {card.image_url ? (
                  <Image
                    src={card.image_url}
                    alt={card.name}
                    width={300}
                    height={400}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <div className="w-full h-full flex items-center justify-center">
                    <span className="text-zinc-500">No Image</span>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Card Details */}
        <div className="lg:col-span-2 space-y-6">
          {/* Basic Information */}
          <Card className="bg-zinc-900 border-zinc-800">
            <CardHeader>
              <CardTitle className="text-white">Card Information</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium text-zinc-400">Card ID</label>
                  <p className="text-white">{card.id}</p>
                </div>
                <div>
                  <label className="text-sm font-medium text-zinc-400">Name</label>
                  <p className="text-white">{card.name}</p>
                </div>
                <div>
                  <label className="text-sm font-medium text-zinc-400">Collection</label>
                  <p className="text-white">{card.collection_name}</p>
                </div>
                <div>
                  <label className="text-sm font-medium text-zinc-400">Collection ID</label>
                  <p className="text-zinc-300 font-mono text-sm">{card.collection_id}</p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Card Properties */}
          <Card className="bg-zinc-900 border-zinc-800">
            <CardHeader>
              <CardTitle className="text-white">Properties</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex flex-wrap gap-3">
                <Badge
                  variant="outline"
                  className={`${getLevelVariant(card.level)} font-semibold`}
                >
                  Level {card.level}
                </Badge>

                {card.animated && (
                  <Badge
                    variant="outline"
                    className="bg-pink-500/20 text-pink-400 border-pink-500/50"
                  >
                    Animated
                  </Badge>
                )}

                {card.promo && (
                  <Badge
                    variant="outline"
                    className="bg-orange-500/20 text-orange-400 border-orange-500/50"
                  >
                    Promotional
                  </Badge>
                )}
              </div>

              {/* Tags */}
              {card.tags && card.tags.length > 0 && (
                <div>
                  <label className="text-sm font-medium text-zinc-400 block mb-2">Tags</label>
                  <div className="flex flex-wrap gap-2">
                    {card.tags.map((tag, index) => (
                      <Badge
                        key={index}
                        variant="outline"
                        className="bg-zinc-700/50 text-zinc-300 border-zinc-600"
                      >
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Timestamps */}
          <Card className="bg-zinc-900 border-zinc-800">
            <CardHeader>
              <CardTitle className="text-white">Timeline</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium text-zinc-400">Created</label>
                  <p className="text-zinc-300">
                    {new Date(card.created_at).toLocaleDateString('en-US', {
                      year: 'numeric',
                      month: 'long',
                      day: 'numeric',
                      hour: '2-digit',
                      minute: '2-digit',
                    })}
                  </p>
                </div>
                <div>
                  <label className="text-sm font-medium text-zinc-400">Last Updated</label>
                  <p className="text-zinc-300">
                    {new Date(card.updated_at).toLocaleDateString('en-US', {
                      year: 'numeric',
                      month: 'long',
                      day: 'numeric',
                      hour: '2-digit',
                      minute: '2-digit',
                    })}
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}