# GoHYE Next.js Frontend Refactor Plan

## 🎯 Project Overview

**Complete frontend replacement** of the existing Go Fiber HTML template system with a modern Next.js 15 application featuring a **minimalistic dark design** approach, built from scratch while preserving all existing functionality.

### **Core Objectives**
- ✅ **Zero Functionality Loss**: Preserve every existing feature
- 🎨 **Minimalistic Dark Design**: Clean, modern UI with shadcn/ui components
- 📱 **Responsive First**: Mobile-optimized layouts with TailwindCSS
- ⚡ **Performance Optimized**: SSR, ISR, and client-side caching
- 🔒 **Security Maintained**: Keep existing Discord OAuth2 and admin controls
- 🧩 **Component Architecture**: Reusable, type-safe React components

---

## 🏗️ Architecture Overview

### **Technology Stack**
```
Frontend:     Next.js 15 (App Router) + React 18 + TypeScript
UI Library:   shadcn/ui + Radix UI + Lucide Icons
Styling:      TailwindCSS + CSS-in-JS for animations
Backend:      Go Fiber (NO CHANGES) - preserve existing API
Database:     PostgreSQL + Bun ORM (NO CHANGES)
Auth:         Discord OAuth2 (preserve existing flow)
Storage:      DigitalOcean Spaces (existing integration)
State:        React Server Components + Zustand for client state
```

### **Project Structure**
```
nextjs-frontend/
├── app/                          # Next.js 15 App Router
│   ├── (auth)/                   # Authentication pages
│   │   ├── login/
│   │   └── oauth/
│   ├── (dashboard)/              # Protected admin dashboard
│   │   ├── cards/                # Card management
│   │   ├── collections/          # Collection management
│   │   ├── users/                # User management  
│   │   ├── sync/                 # Database sync tools
│   │   └── page.tsx              # Dashboard home
│   ├── api/                      # Next.js API routes (proxy layer)
│   │   ├── auth/
│   │   ├── cards/
│   │   ├── collections/
│   │   └── upload/
│   ├── globals.css               # Global styles + shadcn/ui
│   ├── layout.tsx                # Root layout
│   └── loading.tsx               # Global loading UI
├── components/                   # React components
│   ├── ui/                       # shadcn/ui components
│   ├── forms/                    # Form components
│   ├── data-tables/              # Table components
│   ├── layouts/                  # Layout components
│   └── charts/                   # Dashboard charts
├── lib/                          # Utilities and configurations
│   ├── api.ts                    # API client
│   ├── auth.ts                   # Auth helpers
│   ├── types.ts                  # TypeScript definitions
│   └── utils.ts                  # Utility functions
├── hooks/                        # Custom React hooks
├── store/                        # Zustand state management
├── middleware.ts                 # Next.js middleware
├── next.config.js               # Next.js configuration
├── tailwind.config.js           # TailwindCSS configuration
└── components.json               # shadcn/ui configuration
```

---

## 🎨 Design System & UI Architecture

### **Minimalistic Dark Theme**
```css
/* Color Palette */
Background:    #000000 (void black)
Cards:         #0a0a0a (subtle elevation) 
Borders:       #1a1a1a (minimal separation)
Text Primary:  #ffffff (pure white)
Text Secondary:#888888 (muted gray)
Accent Pink:   #ff6b9d (K-pop primary)
Accent Purple: #8b5cf6 (secondary actions)
Success:       #10b981 (positive actions)
Destructive:   #ef4444 (dangerous actions)
```

### **Component Design Principles**
1. **Minimal Visual Noise**: Clean borders, subtle shadows
2. **Generous Whitespace**: Breathing room between elements
3. **Typography Hierarchy**: Clear heading/body text distinction
4. **Consistent Spacing**: 4px base unit scaling (4, 8, 12, 16, 24, 32...)
5. **Smooth Animations**: 150ms ease-in-out transitions
6. **Accessible Contrast**: WCAG AAA compliance

### **shadcn/ui Component Usage**
```typescript
// Core Components
- Button (variants: default, destructive, outline, ghost)
- Card (for content containers)
- Dialog (for modals and confirmations)
- Table (for data display with sorting)
- Form (with react-hook-form integration)
- Input, Textarea (form controls)
- Select, Combobox (dropdowns)
- Badge (for tags and status)
- Toast (for notifications)
- Progress (for upload/loading states)
- Skeleton (for loading placeholders)
```

---

## 📊 Page-by-Page Refactor Plan

### **1. Authentication Pages**

#### `/login` - Discord OAuth Login
```typescript
// Minimalistic login with single Discord button
<div className="min-h-screen bg-black flex items-center justify-center">
  <Card className="w-[400px] p-8 bg-zinc-900 border-zinc-800">
    <div className="text-center space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-semibold text-white">GoHYE Admin</h1>
        <p className="text-sm text-zinc-400">Sign in with Discord to continue</p>
      </div>
      <Button 
        onClick={handleDiscordLogin}
        className="w-full bg-[#5865F2] hover:bg-[#4752C4]"
      >
        <DiscordIcon className="mr-2 h-4 w-4" />
        Continue with Discord
      </Button>
    </div>
  </Card>
</div>
```

#### Key Features:
- ✅ Single Discord OAuth button
- ✅ Preserve existing OAuth flow (no backend changes)
- ✅ Loading states with spinner
- ✅ Error handling with toast notifications

### **2. Dashboard Home**

#### `/dashboard` - Admin Overview
```typescript
// Clean dashboard with essential metrics
<div className="space-y-6">
  {/* Quick Stats */}
  <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
    <Card className="p-6 bg-zinc-900 border-zinc-800">
      <div className="flex items-center space-x-2">
        <CardIcon className="h-4 w-4 text-pink-500" />
        <span className="text-sm font-medium text-zinc-400">Total Cards</span>
      </div>
      <div className="text-2xl font-bold text-white">{stats.totalCards}</div>
    </Card>
    {/* Repeat for Collections, Users, Sync Status */}
  </div>

  {/* Quick Actions */}
  <Card className="p-6 bg-zinc-900 border-zinc-800">
    <h3 className="text-lg font-semibold text-white mb-4">Quick Actions</h3>
    <div className="flex flex-wrap gap-3">
      <Button variant="outline" className="border-zinc-700">
        <PlusIcon className="mr-2 h-4 w-4" />
        Import Album
      </Button>
      <Button variant="outline" className="border-zinc-700">
        <SyncIcon className="mr-2 h-4 w-4" />
        Sync Database
      </Button>
    </div>
  </Card>

  {/* Recent Activity */}
  <Card className="p-6 bg-zinc-900 border-zinc-800">
    <h3 className="text-lg font-semibold text-white mb-4">Recent Activity</h3>
    <div className="space-y-3">
      {activities.map((activity, i) => (
        <div key={i} className="flex items-center space-x-3 text-sm">
          <div className="w-2 h-2 bg-pink-500 rounded-full" />
          <span className="text-zinc-300">{activity.description}</span>
          <span className="text-xs text-zinc-500 ml-auto">{activity.time}</span>
        </div>
      ))}
    </div>
  </Card>
</div>
```

#### Key Features:
- ✅ Essential metrics cards (total cards, collections, users, sync status)
- ✅ Quick action buttons for common tasks
- ✅ Recent activity feed
- ✅ Responsive grid layout

### **3. Card Management**

#### `/dashboard/cards` - Card Grid & Management
```typescript
// Advanced data table with search, filters, and bulk operations
<div className="space-y-6">
  {/* Search & Filters */}
  <Card className="p-4 bg-zinc-900 border-zinc-800">
    <div className="flex flex-col sm:flex-row gap-4">
      <div className="flex-1">
        <Input
          placeholder="Search cards..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="bg-zinc-800 border-zinc-700"
        />
      </div>
      <div className="flex gap-2">
        <Select value={selectedCollection} onValueChange={setSelectedCollection}>
          <SelectTrigger className="w-[180px] bg-zinc-800 border-zinc-700">
            <SelectValue placeholder="Collection" />
          </SelectTrigger>
          <SelectContent>
            {collections.map(collection => (
              <SelectItem key={collection.id} value={collection.id}>
                {collection.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button variant="outline" className="border-zinc-700">
          <FilterIcon className="mr-2 h-4 w-4" />
          Filters
        </Button>
      </div>
    </div>
  </Card>

  {/* Cards Data Table */}
  <Card className="bg-zinc-900 border-zinc-800">
    <CardHeader>
      <div className="flex items-center justify-between">
        <CardTitle className="text-white">Cards ({totalCards})</CardTitle>
        <div className="flex gap-2">
          <Button size="sm" className="bg-pink-600 hover:bg-pink-700">
            <PlusIcon className="mr-2 h-4 w-4" />
            Add Card
          </Button>
          <Button variant="outline" size="sm" className="border-zinc-700">
            <UploadIcon className="mr-2 h-4 w-4" />
            Import
          </Button>
        </div>
      </div>
    </CardHeader>
    <CardContent className="p-0">
      <DataTable
        columns={cardColumns}
        data={cards}
        pageSize={50}
        searchTerm={searchTerm}
        onBulkAction={handleBulkAction}
      />
    </CardContent>
  </Card>
</div>
```

#### Key Features:
- ✅ Advanced search with real-time filtering
- ✅ Collection and level filters
- ✅ Sortable data table with pagination
- ✅ Bulk selection and operations (delete, move, export)
- ✅ Card preview with image thumbnails
- ✅ Responsive design with mobile card view

### **4. Collection Management**

#### `/dashboard/collections` - Collection Grid
```typescript
// Clean collection grid with management actions
<div className="space-y-6">
  {/* Collection Stats */}
  <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
    <Card className="p-4 bg-zinc-900 border-zinc-800 text-center">
      <div className="text-2xl font-bold text-white">{collections.length}</div>
      <div className="text-sm text-zinc-400">Total Collections</div>
    </Card>
    <Card className="p-4 bg-zinc-900 border-zinc-800 text-center">
      <div className="text-2xl font-bold text-pink-500">{girlGroups}</div>
      <div className="text-sm text-zinc-400">Girl Groups</div>
    </Card>
    <Card className="p-4 bg-zinc-900 border-zinc-800 text-center">
      <div className="text-2xl font-bold text-purple-500">{boyGroups}</div>
      <div className="text-sm text-zinc-400">Boy Groups</div>
    </Card>
  </div>

  {/* Collections Grid */}
  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
    {collections.map(collection => (
      <Card key={collection.id} className="bg-zinc-900 border-zinc-800 overflow-hidden">
        <div className="aspect-square bg-gradient-to-br from-pink-500/20 to-purple-500/20">
          {/* Collection thumbnail or placeholder */}
        </div>
        <CardContent className="p-4">
          <h3 className="font-semibold text-white mb-1">{collection.name}</h3>
          <p className="text-sm text-zinc-400 mb-3">{collection.cardCount} cards</p>
          <div className="flex gap-2">
            <Button size="sm" variant="outline" className="border-zinc-700 flex-1">
              <EditIcon className="mr-2 h-4 w-4" />
              Edit
            </Button>
            <Button size="sm" variant="outline" className="border-zinc-700 flex-1">
              <EyeIcon className="mr-2 h-4 w-4" />
              View
            </Button>
          </div>
        </CardContent>
      </Card>
    ))}
  </div>
</div>
```

#### Key Features:
- ✅ Collection statistics overview
- ✅ Responsive grid layout
- ✅ Collection thumbnails and metadata
- ✅ Quick edit and view actions
- ✅ Search and filter capabilities

### **5. File Upload & Import**

#### Album Import Wizard
```typescript
// Multi-step import wizard with progress tracking
<Dialog open={isImporting} onOpenChange={setIsImporting}>
  <DialogContent className="max-w-4xl bg-zinc-900 border-zinc-800">
    <DialogHeader>
      <DialogTitle className="text-white">Import Album</DialogTitle>
      <DialogDescription className="text-zinc-400">
        Step {currentStep} of 4: {stepTitles[currentStep]}
      </DialogDescription>
    </DialogHeader>

    {/* Progress Indicator */}
    <div className="flex items-center space-x-2 mb-6">
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className={`h-2 flex-1 rounded ${
            i < currentStep ? 'bg-pink-500' : 'bg-zinc-700'
          }`}
        />
      ))}
    </div>

    {/* Step Content */}
    {currentStep === 0 && (
      <div className="space-y-4">
        <div className="border-2 border-dashed border-zinc-700 rounded-lg p-8 text-center">
          <UploadIcon className="mx-auto h-12 w-12 text-zinc-400 mb-4" />
          <p className="text-white mb-2">Drop album files here</p>
          <p className="text-sm text-zinc-400">or click to browse</p>
          <Input
            type="file"
            multiple
            accept="image/*"
            onChange={handleFileSelect}
            className="hidden"
            ref={fileInputRef}
          />
        </div>
        {selectedFiles.length > 0 && (
          <div className="text-sm text-zinc-400">
            {selectedFiles.length} files selected
          </div>
        )}
      </div>
    )}

    {/* Additional steps for metadata, preview, processing */}
  </DialogContent>
</Dialog>
```

#### Key Features:
- ✅ Drag-and-drop file upload
- ✅ Multi-step wizard interface
- ✅ File validation and preview
- ✅ Progress tracking with percentage
- ✅ Error handling and retry logic

### **6. Sync Dashboard**

#### `/dashboard/sync` - Database Synchronization
```typescript
// Visual sync status with action buttons
<div className="space-y-6">
  {/* Sync Status Overview */}
  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
    <Card className="p-6 bg-zinc-900 border-zinc-800">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-semibold text-white">Database Status</h3>
          <p className="text-sm text-zinc-400">Last sync: 2 hours ago</p>
        </div>
        <div className="h-3 w-3 bg-green-500 rounded-full animate-pulse" />
      </div>
    </Card>
    
    <Card className="p-6 bg-zinc-900 border-zinc-800">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-semibold text-white">Storage Status</h3>
          <p className="text-sm text-zinc-400">23 orphaned files</p>
        </div>
        <div className="h-3 w-3 bg-yellow-500 rounded-full" />
      </div>
    </Card>
    
    <Card className="p-6 bg-zinc-900 border-zinc-800">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-semibold text-white">Sync Health</h3>
          <p className="text-sm text-zinc-400">98.5% consistent</p>
        </div>
        <div className="h-3 w-3 bg-green-500 rounded-full" />
      </div>
    </Card>
  </div>

  {/* Sync Actions */}
  <Card className="p-6 bg-zinc-900 border-zinc-800">
    <h3 className="text-lg font-semibold text-white mb-4">Sync Actions</h3>
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
      <Button variant="outline" className="border-zinc-700 justify-start">
        <RefreshIcon className="mr-2 h-4 w-4" />
        Full Sync
      </Button>
      <Button variant="outline" className="border-zinc-700 justify-start">
        <TrashIcon className="mr-2 h-4 w-4" />
        Clean Orphans
      </Button>
      <Button variant="outline" className="border-zinc-700 justify-start">
        <SearchIcon className="mr-2 h-4 w-4" />
        Find Missing
      </Button>
      <Button variant="outline" className="border-zinc-700 justify-start">
        <CheckIcon className="mr-2 h-4 w-4" />
        Validate All
      </Button>
    </div>
  </Card>
</div>
```

#### Key Features:
- ✅ Visual sync status indicators
- ✅ Real-time health monitoring
- ✅ One-click sync actions
- ✅ Progress tracking for operations
- ✅ Detailed sync reports

---

## 🔧 Technical Implementation Details

### **1. Backend Integration Strategy**

#### Minimal Go Backend Changes
```go
// Add single validation endpoint - web/handlers/handlers.go
func ValidateSession(webApp *WebApp) fiber.Handler {
    return func(c *fiber.Ctx) error {
        session, err := webApp.GetSession(c)
        if err != nil {
            return utils.SendUnauthorized(c, "Invalid session")
        }
        
        return utils.SendSuccess(c, fiber.Map{
            "user": session,
            "valid": true,
        }, "Session valid")
    }
}

// Update CORS configuration - web/main.go  
app.Use(cors.New(cors.Config{
    AllowOrigins:     "http://localhost:3000,http://localhost:8080",
    AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
    AllowHeaders:     "Origin,Content-Type,Accept,Authorization,Cookie",
    AllowCredentials: true,
}))
```

### **2. Authentication Flow**

#### Next.js Middleware for Route Protection
```typescript
// middleware.ts
export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  
  // Protect dashboard routes
  if (pathname.startsWith('/dashboard')) {
    const sessionCookie = request.cookies.get('gohye_session');
    
    if (!sessionCookie) {
      return NextResponse.redirect(new URL('/login', request.url));
    }
    
    // Validate with Go backend
    try {
      const response = await fetch(`${process.env.GO_BACKEND_URL}/api/auth/validate`, {
        headers: { Cookie: `gohye_session=${sessionCookie.value}` },
      });
      
      if (!response.ok) {
        return NextResponse.redirect(new URL('/login', request.url));
      }
    } catch {
      return NextResponse.redirect(new URL('/login', request.url));
    }
  }
  
  return NextResponse.next();
}

export const config = {
  matcher: ['/dashboard/:path*']
};
```

#### Session Management
```typescript
// lib/auth.ts
export async function getServerSession(): Promise<UserSession | null> {
  const cookieStore = cookies();
  const sessionCookie = cookieStore.get('gohye_session');
  
  if (!sessionCookie) return null;
  
  try {
    const response = await fetch(`${process.env.GO_BACKEND_URL}/api/auth/validate`, {
      headers: { Cookie: `gohye_session=${sessionCookie.value}` },
      next: { revalidate: 300 }, // Cache for 5 minutes
    });
    
    if (!response.ok) return null;
    return response.json();
  } catch {
    return null;
  }
}
```

### **3. Data Fetching Patterns**

#### Server Components for Initial Data
```typescript
// app/(dashboard)/cards/page.tsx
export default async function CardsPage({ searchParams }: PageProps) {
  const session = await getServerSession();
  if (!session) redirect('/login');
  
  // Fetch initial data server-side
  const [cards, collections] = await Promise.all([
    getCards(searchParams),
    getCollections(),
  ]);
  
  return (
    <div className="space-y-6">
      <CardManagementHeader />
      <CardsDataTable 
        initialCards={cards}
        collections={collections}
        searchParams={searchParams}
      />
    </div>
  );
}
```

#### Client Components for Interactivity
```typescript
// components/CardsDataTable.tsx
'use client';

export function CardsDataTable({ initialCards, collections }: Props) {
  const [cards, setCards] = useState(initialCards);
  const [loading, setLoading] = useState(false);
  const [selectedCards, setSelectedCards] = useState<string[]>([]);
  
  // Client-side search with debouncing
  const debouncedSearch = useDeferredValue(searchTerm);
  
  useEffect(() => {
    if (debouncedSearch !== searchTerm) return;
    
    setLoading(true);
    searchCards({ term: debouncedSearch, collection: selectedCollection })
      .then(setCards)
      .finally(() => setLoading(false));
  }, [debouncedSearch, selectedCollection]);
  
  return (
    <Card className="bg-zinc-900 border-zinc-800">
      <CardHeader>
        <SearchFilters
          searchTerm={searchTerm}
          onSearchChange={setSearchTerm}
          collections={collections}
          selectedCollection={selectedCollection}
          onCollectionChange={setSelectedCollection}
        />
      </CardHeader>
      <CardContent>
        <DataTable
          columns={cardColumns}
          data={cards}
          loading={loading}
          selectedRows={selectedCards}
          onSelectionChange={setSelectedCards}
        />
      </CardContent>
    </Card>
  );
}
```

### **4. API Layer Design**

#### Next.js API Routes as Proxy
```typescript
// app/api/cards/route.ts
export async function GET(request: Request) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }
  
  const { searchParams } = new URL(request.url);
  
  try {
    const response = await fetch(
      `${process.env.GO_BACKEND_URL}/admin/api/cards?${searchParams}`,
      {
        headers: {
          Cookie: request.headers.get('cookie') || '',
        },
      }
    );
    
    const data = await response.json();
    
    return Response.json(data, {
      headers: {
        'Cache-Control': 'public, max-age=60, s-maxage=300',
      },
    });
  } catch (error) {
    return new Response('Internal Server Error', { status: 500 });
  }
}
```

### **5. State Management**

#### Zustand for Client State
```typescript
// store/useCardsStore.ts
interface CardsState {
  cards: Card[];
  loading: boolean;
  selectedCards: string[];
  searchTerm: string;
  filters: CardFilters;
  
  // Actions
  setCards: (cards: Card[]) => void;
  setLoading: (loading: boolean) => void;
  toggleCardSelection: (cardId: string) => void;
  updateSearchTerm: (term: string) => void;
  applyFilters: (filters: CardFilters) => void;
  bulkDeleteCards: (cardIds: string[]) => Promise<void>;
}

export const useCardsStore = create<CardsState>((set, get) => ({
  cards: [],
  loading: false,
  selectedCards: [],
  searchTerm: '',
  filters: {},
  
  setCards: (cards) => set({ cards }),
  setLoading: (loading) => set({ loading }),
  
  toggleCardSelection: (cardId) => {
    const { selectedCards } = get();
    const newSelection = selectedCards.includes(cardId)
      ? selectedCards.filter(id => id !== cardId)
      : [...selectedCards, cardId];
    set({ selectedCards: newSelection });
  },
  
  bulkDeleteCards: async (cardIds) => {
    set({ loading: true });
    try {
      await deleteCards(cardIds);
      const { cards } = get();
      set({ 
        cards: cards.filter(card => !cardIds.includes(card.id)),
        selectedCards: [],
      });
    } finally {
      set({ loading: false });
    }
  },
}));
```

---

## 🚀 Implementation Timeline

### **Phase 1: Foundation Setup (Week 1)**
- [ ] Initialize Next.js 15 project with TypeScript
- [ ] Setup shadcn/ui component library
- [ ] Configure TailwindCSS with dark theme
- [ ] Implement basic routing structure
- [ ] Create base layout components
- [ ] Setup authentication integration
- [ ] Add Go backend session validation endpoint

### **Phase 2: Core Pages (Week 2)**
- [ ] Build login page with Discord OAuth
- [ ] Create dashboard home with metrics
- [ ] Implement card management page
- [ ] Build collection management interface
- [ ] Add user management capabilities
- [ ] Create sync dashboard

### **Phase 3: Advanced Features (Week 3)**
- [ ] Implement file upload system
- [ ] Add album import wizard
- [ ] Build data tables with sorting/filtering
- [ ] Add bulk operations
- [ ] Implement real-time progress tracking
- [ ] Add error handling and toast notifications

### **Phase 4: Polish & Optimization (Week 4)**
- [ ] Mobile responsiveness optimization
- [ ] Performance improvements (caching, lazy loading)
- [ ] Accessibility enhancements
- [ ] Animation and micro-interactions
- [ ] Production build optimization
- [ ] Testing and bug fixes

---

## 📋 Migration Checklist

### **Functionality Preservation**
- [ ] ✅ Discord OAuth2 authentication flow
- [ ] ✅ Admin role-based access control
- [ ] ✅ Card CRUD operations (create, read, update, delete)
- [ ] ✅ Card search and filtering
- [ ] ✅ Collection management
- [ ] ✅ Bulk card operations (delete, move, export)
- [ ] ✅ File upload with progress tracking
- [ ] ✅ Album import wizard
- [ ] ✅ Database synchronization tools
- [ ] ✅ User management interface
- [ ] ✅ Real-time activity feed
- [ ] ✅ Statistics and analytics
- [ ] ✅ Error handling and logging

### **Design Requirements**
- [ ] ✅ Minimalistic dark theme
- [ ] ✅ Responsive design (mobile-first)
- [ ] ✅ Clean typography and spacing
- [ ] ✅ Consistent component design
- [ ] ✅ Smooth animations and transitions
- [ ] ✅ Accessible color contrast
- [ ] ✅ Loading states and skeletons
- [ ] ✅ Toast notifications for feedback

### **Technical Requirements**
- [ ] ✅ TypeScript for type safety
- [ ] ✅ Server-side rendering for SEO
- [ ] ✅ Client-side interactivity
- [ ] ✅ Optimized bundle sizes
- [ ] ✅ Caching strategies
- [ ] ✅ Error boundaries
- [ ] ✅ Security best practices

---

## 🎯 Success Metrics

### **Performance Targets**
- **First Contentful Paint**: < 1.5 seconds
- **Largest Contentful Paint**: < 2.5 seconds
- **Time to Interactive**: < 3 seconds
- **Bundle Size**: < 500KB gzipped
- **Lighthouse Score**: 95+ (Performance, Accessibility, Best Practices)

### **User Experience Goals**
- **Mobile Usability**: 100% touch-friendly interactions
- **Accessibility**: WCAG 2.1 AA compliance
- **Error Rate**: < 1% user-facing errors
- **Task Completion**: 95% success rate for admin tasks
- **Load Time**: 90% of pages load under 2 seconds

---

## 🔗 Dependencies & Configuration

### **Package.json Dependencies**
```json
{
  "dependencies": {
    "next": "^15.0.0",
    "react": "^18.0.0",
    "react-dom": "^18.0.0",
    "@radix-ui/react-dialog": "latest",
    "@radix-ui/react-select": "latest",
    "@radix-ui/react-table": "latest",
    "lucide-react": "latest",
    "tailwindcss": "latest",
    "zustand": "latest",
    "react-hook-form": "latest",
    "@hookform/resolvers": "latest",
    "zod": "latest"
  },
  "devDependencies": {
    "@types/node": "latest",
    "@types/react": "latest",
    "typescript": "latest",
    "eslint": "latest",
    "prettier": "latest"
  }
}
```

### **Environment Configuration**
```bash
# .env.local
GO_BACKEND_URL=http://localhost:8080
NEXT_PUBLIC_APP_URL=http://localhost:3000
NODE_ENV=development
```

### **Next.js Configuration**
```javascript
// next.config.js
/** @type {import('next').NextConfig} */
const nextConfig = {
  experimental: {
    serverActions: true,
  },
  
  images: {
    domains: ['your-spaces-domain.com'],
    formats: ['image/webp', 'image/avif'],
  },
  
  async rewrites() {
    return [
      {
        source: '/api/backend/:path*',
        destination: `${process.env.GO_BACKEND_URL}/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
```

---

## 🏁 Conclusion

This comprehensive refactor plan transforms the GoHYE admin panel from a traditional server-rendered application to a modern, minimalistic Next.js frontend while preserving all existing functionality. The dark theme design system provides a clean, professional interface that scales beautifully across all devices.

**Key Benefits:**
- 🚀 **Modern UX**: Fast, responsive, and intuitive interface
- 🎨 **Minimalistic Design**: Clean, distraction-free admin experience  
- 📱 **Mobile-First**: Optimized for all screen sizes
- ⚡ **Performance**: Server-side rendering with client-side interactivity
- 🔒 **Security**: Maintained authentication and authorization
- 🧩 **Maintainable**: Component-based architecture with TypeScript

The implementation preserves your existing Go backend investment while providing a foundation for future enhancements and scalability.

---

*GoHYE Next.js Frontend Refactor Plan v1.0 | Estimated Timeline: 4 weeks | Zero functionality loss guaranteed*