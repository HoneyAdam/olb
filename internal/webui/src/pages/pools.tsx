import { useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Layers, Plus, Search, Trash2, Edit, Activity, Clock } from "lucide-react"
import { toast } from "sonner"
import type { Pool, Backend } from "@/types"

const mockPools: Pool[] = [
  {
    id: "1",
    name: "api-pool",
    algorithm: "round_robin",
    backends: [
      { id: "b1", address: "10.0.1.10:8080", weight: 1, status: "up", health: "healthy", response_time_ms: 12, active_connections: 45, total_requests: 1250000 },
      { id: "b2", address: "10.0.1.11:8080", weight: 1, status: "up", health: "healthy", response_time_ms: 15, active_connections: 38, total_requests: 1180000 },
      { id: "b3", address: "10.0.1.12:8080", weight: 1, status: "down", health: "unhealthy", response_time_ms: 0, active_connections: 0, total_requests: 500000 },
    ],
    health_check: { enabled: true, type: "http", path: "/health", interval: "10s" },
    total_requests: 2930000,
    active_connections: 83,
  },
  {
    id: "2",
    name: "web-pool",
    algorithm: "least_connections",
    backends: [
      { id: "b4", address: "10.0.2.10:3000", weight: 2, status: "up", health: "healthy", response_time_ms: 8, active_connections: 120, total_requests: 5000000 },
      { id: "b5", address: "10.0.2.11:3000", weight: 2, status: "up", health: "healthy", response_time_ms: 10, active_connections: 98, total_requests: 4800000 },
    ],
    health_check: { enabled: true, type: "http", path: "/", interval: "5s" },
    total_requests: 9800000,
    active_connections: 218,
  },
  {
    id: "3",
    name: "grpc-pool",
    algorithm: "ip_hash",
    backends: [
      { id: "b6", address: "10.0.3.10:50051", weight: 1, status: "up", health: "healthy", response_time_ms: 5, active_connections: 25, total_requests: 800000 },
      { id: "b7", address: "10.0.3.11:50051", weight: 1, status: "up", health: "healthy", response_time_ms: 6, active_connections: 22, total_requests: 750000 },
    ],
    health_check: { enabled: true, type: "grpc", path: "", interval: "10s" },
    total_requests: 1550000,
    active_connections: 47,
  },
]

const algorithmLabels: Record<string, string> = {
  round_robin: "Round Robin",
  least_connections: "Least Connections",
  ip_hash: "IP Hash",
  weighted_round_robin: "Weighted Round Robin",
  random: "Random",
  first: "First",
}

const algorithms = [
  { value: "round_robin", label: "Round Robin" },
  { value: "least_connections", label: "Least Connections" },
  { value: "ip_hash", label: "IP Hash" },
  { value: "weighted_round_robin", label: "Weighted Round Robin" },
  { value: "random", label: "Random" },
  { value: "first", label: "First" },
]

export function PoolsPage() {
  const [pools, setPools] = useState<Pool[]>(mockPools)
  const [search, setSearch] = useState("")
  const [selectedPool, setSelectedPool] = useState<Pool | null>(mockPools[0])

  // Create Pool Dialog State
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [newPool, setNewPool] = useState({
    name: "",
    algorithm: "round_robin",
    healthCheckEnabled: true,
    healthCheckType: "http",
    healthCheckPath: "/health",
    healthCheckInterval: "10s",
  })

  // Add Backend Dialog State
  const [backendDialogOpen, setBackendDialogOpen] = useState(false)
  const [newBackend, setNewBackend] = useState({
    address: "",
    weight: 1,
  })

  const filteredPools = pools.filter(p =>
    p.name.toLowerCase().includes(search.toLowerCase())
  )

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'up': return 'bg-green-500'
      case 'down': return 'bg-red-500'
      case 'draining': return 'bg-amber-500'
      default: return 'bg-gray-500'
    }
  }

  const getHealthBadge = (health: string) => {
    switch (health) {
      case 'healthy': return <Badge variant="outline" className="text-green-600 border-green-600">Healthy</Badge>
      case 'unhealthy': return <Badge variant="destructive">Unhealthy</Badge>
      default: return <Badge variant="secondary">Unknown</Badge>
    }
  }

  const handleCreatePool = () => {
    const pool: Pool = {
      id: Math.random().toString(36).substr(2, 9),
      name: newPool.name,
      algorithm: newPool.algorithm,
      backends: [],
      health_check: newPool.healthCheckEnabled ? {
        enabled: true,
        type: newPool.healthCheckType,
        path: newPool.healthCheckPath,
        interval: newPool.healthCheckInterval,
      } : undefined,
      total_requests: 0,
      active_connections: 0,
    }
    setPools([...pools, pool])
    setCreateDialogOpen(false)
    setNewPool({
      name: "",
      algorithm: "round_robin",
      healthCheckEnabled: true,
      healthCheckType: "http",
      healthCheckPath: "/health",
      healthCheckInterval: "10s",
    })
    toast.success(`Pool "${pool.name}" created successfully`)
  }

  const handleAddBackend = () => {
    if (!selectedPool) return
    const backend: Backend = {
      id: Math.random().toString(36).substr(2, 9),
      address: newBackend.address,
      weight: newBackend.weight,
      status: "up",
      health: "unknown",
      response_time_ms: 0,
      active_connections: 0,
      total_requests: 0,
    }
    const updatedPool = { ...selectedPool, backends: [...selectedPool.backends, backend] }
    setPools(pools.map(p => p.id === selectedPool.id ? updatedPool : p))
    setSelectedPool(updatedPool)
    setBackendDialogOpen(false)
    setNewBackend({ address: "", weight: 1 })
    toast.success(`Backend "${backend.address}" added successfully`)
  }

  const handleDeletePool = (poolId: string) => {
    setPools(pools.filter(p => p.id !== poolId))
    if (selectedPool?.id === poolId) {
      setSelectedPool(null)
    }
    toast.success("Pool deleted successfully")
  }

  const handleDeleteBackend = (backendId: string) => {
    if (!selectedPool) return
    const updatedPool = { ...selectedPool, backends: selectedPool.backends.filter(b => b.id !== backendId) }
    setPools(pools.map(p => p.id === selectedPool.id ? updatedPool : p))
    setSelectedPool(updatedPool)
    toast.success("Backend removed successfully")
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Pools</h1>
          <p className="text-muted-foreground">Manage backend pools and load balancing</p>
        </div>
        <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Create Pool
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[500px]">
            <DialogHeader>
              <DialogTitle>Create New Pool</DialogTitle>
              <DialogDescription>
                Configure a new backend pool with load balancing settings.
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="name">Pool Name</Label>
                <Input
                  id="name"
                  placeholder="e.g., api-pool"
                  value={newPool.name}
                  onChange={(e) => setNewPool({ ...newPool, name: e.target.value })}
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="algorithm">Algorithm</Label>
                <Select
                  value={newPool.algorithm}
                  onValueChange={(value: string) => setNewPool({ ...newPool, algorithm: value })}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select algorithm" />
                  </SelectTrigger>
                  <SelectContent>
                    {algorithms.map((algo) => (
                      <SelectItem key={algo.value} value={algo.value}>
                        {algo.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center justify-between">
                <Label htmlFor="health-check">Enable Health Checks</Label>
                <Switch
                  id="health-check"
                  checked={newPool.healthCheckEnabled}
                  onCheckedChange={(checked) => setNewPool({ ...newPool, healthCheckEnabled: checked })}
                />
              </div>
              {newPool.healthCheckEnabled && (
                <>
                  <div className="grid gap-2">
                    <Label htmlFor="hc-type">Health Check Type</Label>
                    <Select
                      value={newPool.healthCheckType}
                      onValueChange={(value: string) => setNewPool({ ...newPool, healthCheckType: value })}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="http">HTTP</SelectItem>
                        <SelectItem value="tcp">TCP</SelectItem>
                        <SelectItem value="grpc">gRPC</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="hc-path">Health Check Path</Label>
                    <Input
                      id="hc-path"
                      placeholder="/health"
                      value={newPool.healthCheckPath}
                      onChange={(e) => setNewPool({ ...newPool, healthCheckPath: e.target.value })}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="hc-interval">Interval</Label>
                    <Select
                      value={newPool.healthCheckInterval}
                      onValueChange={(value: string) => setNewPool({ ...newPool, healthCheckInterval: value })}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="5s">5 seconds</SelectItem>
                        <SelectItem value="10s">10 seconds</SelectItem>
                        <SelectItem value="30s">30 seconds</SelectItem>
                        <SelectItem value="1m">1 minute</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </>
              )}
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button onClick={handleCreatePool} disabled={!newPool.name}>
                Create Pool
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <div className="flex gap-4">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search pools..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-10"
          />
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-4">
          {filteredPools.map((pool) => (
            <Card
              key={pool.id}
              className={`cursor-pointer transition-colors hover:bg-accent ${selectedPool?.id === pool.id ? 'border-primary' : ''}`}
              onClick={() => setSelectedPool(pool)}
            >
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-lg bg-primary/10">
                      <Layers className="h-5 w-5 text-primary" />
                    </div>
                    <div>
                      <CardTitle className="text-base">{pool.name}</CardTitle>
                      <CardDescription>{algorithmLabels[pool.algorithm] || pool.algorithm}</CardDescription>
                    </div>
                  </div>
                  <div className="flex gap-1">
                    <Button variant="ghost" size="icon" className="h-8 w-8">
                      <Edit className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-destructive"
                      onClick={(e) => {
                        e.stopPropagation()
                        handleDeletePool(pool.id)
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <span className="text-muted-foreground">Backends:</span>
                    <span className="ml-2 font-medium">{pool.backends.length}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Healthy:</span>
                    <span className="ml-2 font-medium text-green-600">
                      {pool.backends.filter(b => b.health === 'healthy').length}
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="lg:col-span-2">
          {selectedPool ? (
            <Tabs defaultValue="backends" className="space-y-4">
              <TabsList>
                <TabsTrigger value="backends">Backends</TabsTrigger>
                <TabsTrigger value="settings">Settings</TabsTrigger>
                <TabsTrigger value="stats">Statistics</TabsTrigger>
              </TabsList>

              <TabsContent value="backends" className="space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="text-lg font-medium">Backends</h3>
                  <Dialog open={backendDialogOpen} onOpenChange={setBackendDialogOpen}>
                    <DialogTrigger asChild>
                      <Button size="sm">
                        <Plus className="mr-2 h-4 w-4" />
                        Add Backend
                      </Button>
                    </DialogTrigger>
                    <DialogContent className="sm:max-w-[400px]">
                      <DialogHeader>
                        <DialogTitle>Add Backend</DialogTitle>
                        <DialogDescription>
                          Add a new backend server to this pool.
                        </DialogDescription>
                      </DialogHeader>
                      <div className="grid gap-4 py-4">
                        <div className="grid gap-2">
                          <Label htmlFor="address">Backend Address</Label>
                          <Input
                            id="address"
                            placeholder="e.g., 10.0.1.10:8080"
                            value={newBackend.address}
                            onChange={(e) => setNewBackend({ ...newBackend, address: e.target.value })}
                          />
                        </div>
                        <div className="grid gap-2">
                          <Label htmlFor="weight">Weight</Label>
                          <Input
                            id="weight"
                            type="number"
                            min={1}
                            value={newBackend.weight}
                            onChange={(e) => setNewBackend({ ...newBackend, weight: parseInt(e.target.value) || 1 })}
                          />
                        </div>
                      </div>
                      <DialogFooter>
                        <Button variant="outline" onClick={() => setBackendDialogOpen(false)}>
                          Cancel
                        </Button>
                        <Button onClick={handleAddBackend} disabled={!newBackend.address}>
                          Add Backend
                        </Button>
                      </DialogFooter>
                    </DialogContent>
                  </Dialog>
                </div>

                <div className="grid gap-4">
                  {selectedPool.backends.map((backend) => (
                    <Card key={backend.id}>
                      <CardContent className="p-4">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-4">
                            <div className={`h-3 w-3 rounded-full ${getStatusColor(backend.status)}`} />
                            <div>
                              <div className="font-medium">{backend.address}</div>
                              <div className="text-sm text-muted-foreground">
                                Weight: {backend.weight}
                              </div>
                            </div>
                          </div>
                          <div className="flex items-center gap-4">
                            {getHealthBadge(backend.health)}
                            <div className="text-right text-sm">
                              <div className="flex items-center gap-1 text-muted-foreground">
                                <Activity className="h-3 w-3" />
                                {backend.active_connections} conn
                              </div>
                              <div className="flex items-center gap-1 text-muted-foreground">
                                <Clock className="h-3 w-3" />
                                {backend.response_time_ms}ms
                              </div>
                            </div>
                            <div className="flex gap-1">
                              <Button variant="ghost" size="icon" className="h-8 w-8">
                                <Edit className="h-4 w-4" />
                              </Button>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8 text-destructive"
                                onClick={() => handleDeleteBackend(backend.id)}
                              >
                                <Trash2 className="h-4 w-4" />
                              </Button>
                            </div>
                          </div>
                        </div>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              </TabsContent>

              <TabsContent value="settings">
                <Card>
                  <CardHeader>
                    <CardTitle>Pool Settings</CardTitle>
                    <CardDescription>Configure load balancing algorithm and health checks</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="grid gap-4 md:grid-cols-2">
                      <div>
                        <label className="text-sm font-medium">Algorithm</label>
                        <div className="mt-1 text-sm text-muted-foreground">
                          {algorithmLabels[selectedPool.algorithm] || selectedPool.algorithm}
                        </div>
                      </div>
                      <div>
                        <label className="text-sm font-medium">Health Check</label>
                        <div className="mt-1 text-sm text-muted-foreground">
                          {selectedPool.health_check?.enabled ? 'Enabled' : 'Disabled'}
                        </div>
                      </div>
                      {selectedPool.health_check?.enabled && (
                        <>
                          <div>
                            <label className="text-sm font-medium">Check Type</label>
                            <div className="mt-1 text-sm text-muted-foreground uppercase">
                              {selectedPool.health_check.type}
                            </div>
                          </div>
                          <div>
                            <label className="text-sm font-medium">Interval</label>
                            <div className="mt-1 text-sm text-muted-foreground">
                              {selectedPool.health_check.interval}
                            </div>
                          </div>
                          {selectedPool.health_check.path && (
                            <div>
                              <label className="text-sm font-medium">Path</label>
                              <div className="mt-1 text-sm text-muted-foreground">
                                {selectedPool.health_check.path}
                              </div>
                            </div>
                          )}
                        </>
                      )}
                    </div>
                  </CardContent>
                </Card>
              </TabsContent>

              <TabsContent value="stats">
                <div className="grid gap-4 md:grid-cols-2">
                  <Card>
                    <CardHeader className="pb-2">
                      <CardTitle className="text-sm font-medium">Total Requests</CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className="text-2xl font-bold">{selectedPool.total_requests.toLocaleString()}</div>
                    </CardContent>
                  </Card>
                  <Card>
                    <CardHeader className="pb-2">
                      <CardTitle className="text-sm font-medium">Active Connections</CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className="text-2xl font-bold">{selectedPool.active_connections}</div>
                    </CardContent>
                  </Card>
                </div>
              </TabsContent>
            </Tabs>
          ) : (
            <div className="flex h-64 items-center justify-center text-muted-foreground">
              Select a pool to view details
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
