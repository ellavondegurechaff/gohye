import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export function CardsTableSkeleton() {
  return (
    <Card className="bg-zinc-900 border-zinc-800">
      <CardHeader>
        <Skeleton className="h-6 w-32" />
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="bg-zinc-800 rounded-lg p-4 border border-zinc-700">
              <Skeleton className="aspect-[3/4] rounded-lg mb-3" />
              <Skeleton className="h-4 w-full mb-1" />
              <Skeleton className="h-3 w-20 mb-2" />
              <div className="flex items-center justify-between">
                <Skeleton className="h-6 w-8" />
                <Skeleton className="h-6 w-16" />
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}