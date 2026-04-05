import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Clock, Globe, Lock, Zap, Server, ScrollText, Shield, Code, Settings2 } from "lucide-react"
import { toast } from "sonner"
import { cn } from "@/lib/utils"

interface MiddlewareItem {
  id: string
  name: string
  description: string
  enabled: boolean
  icon: React.ComponentType<{ className?: string }>
  category: "security" | "performance" | "traffic" | "observability"
  config?: Record<string, any>
}

const middlewareList: MiddlewareItem[] = [
  {
    id: "rate_limit",
    name: "Rate Limiting",
    description: "Limit requests per IP or user",
    enabled: true,
    icon: Clock,
    category: "traffic",
    config: { requestsPerSecond: 100, burstSize: 150 }
  },
  {
    id: "cors",
    name: "CORS",
    description: "Cross-Origin Resource Sharing",
    enabled: true,
    icon: Globe,
    category: "security",
    config: { allowedOrigins: "*", allowedMethods: "GET,POST,PUT,DELETE" }
  },
  {
    id: "jwt",
    name: "JWT Auth",
    description: "JSON Web Token authentication",
    enabled: false,
    icon: Lock,
    category: "security",
    config: { secret: "", algorithm: "HS256" }
  },
  {
    id: "compression",
    name: "Compression",
    description: "Gzip/Brotli response compression",
    enabled: true,
    icon: Zap,
    category: "performance",
    config: { level: 6, types: "text/html,text/css,application/javascript" }
  },
  {
    id: "cache",
    name: "HTTP Cache",
    description: "Response caching with TTL",
    enabled: false,
    icon: Server,
    category: "performance",
    config: { ttl: 300, maxSize: "100MB" }
  },
  {
    id: "logging",
    name: "Access Logging",
    description: "Request/response logging",
    enabled: true,
    icon: ScrollText,
    category: "observability",
    config: { format: "combined", output: "stdout" }
  },
  {
    id: "api_key",
    name: "API Key Auth",
    description: "API key based authentication",
    enabled: false,
    icon: Shield,
    category: "security",
    config: { header: "X-API-Key", keys: [] }
  },
  {
    id: "transform",
    name: "Request Transform",
    description: "Modify request/response headers and body",
    enabled: false,
    icon: Code,
    category: "traffic",
    config: { headers: {}, body: "" }
  },
]

export function MiddlewarePage() {
  const [middlewares, setMiddlewares] = useState<MiddlewareItem[]>(middlewareList)
  const [selectedCategory, setSelectedCategory] = useState<string>("all")
  const [configDialogOpen, setConfigDialogOpen] = useState(false)
  const [selectedMiddleware, setSelectedMiddleware] = useState<MiddlewareItem | null>(null)
  const [configValues, setConfigValues] = useState<Record<string, any>>({})

  const toggleMiddleware = (id: string) => {
    setMiddlewares(prev => prev.map(m =>
      m.id === id ? { ...m, enabled: !m.enabled } : m
    ))
    const mw = middlewares.find(m => m.id === id)
    toast.success(`${mw?.name} ${mw?.enabled ? 'disabled' : 'enabled'}`)
  }

  const openConfigDialog = (middleware: MiddlewareItem) => {
    setSelectedMiddleware(middleware)
    setConfigValues(middleware.config || {})
    setConfigDialogOpen(true)
  }

  const saveConfig = () => {
    if (!selectedMiddleware) return
    setMiddlewares(prev => prev.map(m =>
      m.id === selectedMiddleware.id ? { ...m, config: configValues } : m
    ))
    setConfigDialogOpen(false)
    toast.success(`${selectedMiddleware.name} configuration saved`)
  }

  const filteredMiddleware = selectedCategory === "all"
    ? middlewares
    : middlewares.filter(m => m.category === selectedCategory)

  const categories = [
    { id: "all", label: "All" },
    { id: "security", label: "Security" },
    { id: "performance", label: "Performance" },
    { id: "traffic", label: "Traffic" },
    { id: "observability", label: "Observability" },
  ]

  const categoryColors: Record<string, string> = {
    security: "bg-red-500/10 text-red-600",
    performance: "bg-green-500/10 text-green-600",
    traffic: "bg-blue-500/10 text-blue-600",
    observability: "bg-purple-500/10 text-purple-600",
  }

  const renderConfigFields = () => {
    if (!selectedMiddleware) return null

    switch (selectedMiddleware.id) {
      case "rate_limit":
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="rl-requests">Requests Per Second</Label>
              <Input
                id="rl-requests"
                type="number"
                value={configValues.requestsPerSecond || 100}
                onChange={(e) => setConfigValues({ ...configValues, requestsPerSecond: parseInt(e.target.value) })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="rl-burst">Burst Size</Label>
              <Input
                id="rl-burst"
                type="number"
                value={configValues.burstSize || 150}
                onChange={(e) => setConfigValues({ ...configValues, burstSize: parseInt(e.target.value) })}
              />
            </div>
          </>
        )
      case "cors":
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="cors-origins">Allowed Origins</Label>
              <Input
                id="cors-origins"
                placeholder="* or https://example.com"
                value={configValues.allowedOrigins || "*"}
                onChange={(e) => setConfigValues({ ...configValues, allowedOrigins: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="cors-methods">Allowed Methods</Label>
              <Input
                id="cors-methods"
                value={configValues.allowedMethods || "GET,POST,PUT,DELETE"}
                onChange={(e) => setConfigValues({ ...configValues, allowedMethods: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="cors-headers">Allowed Headers</Label>
              <Input
                id="cors-headers"
                placeholder="Content-Type,Authorization"
                value={configValues.allowedHeaders || ""}
                onChange={(e) => setConfigValues({ ...configValues, allowedHeaders: e.target.value })}
              />
            </div>
          </>
        )
      case "jwt":
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="jwt-secret">JWT Secret</Label>
              <Input
                id="jwt-secret"
                type="password"
                placeholder="Enter secret key"
                value={configValues.secret || ""}
                onChange={(e) => setConfigValues({ ...configValues, secret: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="jwt-algo">Algorithm</Label>
              <Select
                value={configValues.algorithm || "HS256"}
                onValueChange={(value: string) => setConfigValues({ ...configValues, algorithm: value })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="HS256">HS256</SelectItem>
                  <SelectItem value="HS384">HS384</SelectItem>
                  <SelectItem value="HS512">HS512</SelectItem>
                  <SelectItem value="RS256">RS256</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="jwt-header">Header Name</Label>
              <Input
                id="jwt-header"
                value={configValues.header || "Authorization"}
                onChange={(e) => setConfigValues({ ...configValues, header: e.target.value })}
              />
            </div>
          </>
        )
      case "compression":
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="comp-level">Compression Level (1-9)</Label>
              <Input
                id="comp-level"
                type="number"
                min={1}
                max={9}
                value={configValues.level || 6}
                onChange={(e) => setConfigValues({ ...configValues, level: parseInt(e.target.value) })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="comp-types">Content Types</Label>
              <Textarea
                id="comp-types"
                value={configValues.types || "text/html,text/css,application/javascript"}
                onChange={(e) => setConfigValues({ ...configValues, types: e.target.value })}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="comp-brotli">Enable Brotli</Label>
              <Switch
                id="comp-brotli"
                checked={configValues.brotli || false}
                onCheckedChange={(checked) => setConfigValues({ ...configValues, brotli: checked })}
              />
            </div>
          </>
        )
      case "cache":
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="cache-ttl">TTL (seconds)</Label>
              <Input
                id="cache-ttl"
                type="number"
                value={configValues.ttl || 300}
                onChange={(e) => setConfigValues({ ...configValues, ttl: parseInt(e.target.value) })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="cache-size">Max Size</Label>
              <Input
                id="cache-size"
                value={configValues.maxSize || "100MB"}
                onChange={(e) => setConfigValues({ ...configValues, maxSize: e.target.value })}
              />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="cache-vary">Vary by Headers</Label>
              <Switch
                id="cache-vary"
                checked={configValues.varyByHeaders || false}
                onCheckedChange={(checked) => setConfigValues({ ...configValues, varyByHeaders: checked })}
              />
            </div>
          </>
        )
      case "logging":
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="log-format">Log Format</Label>
              <Select
                value={configValues.format || "combined"}
                onValueChange={(value: string) => setConfigValues({ ...configValues, format: value })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="common">Common</SelectItem>
                  <SelectItem value="combined">Combined</SelectItem>
                  <SelectItem value="json">JSON</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="log-output">Output</Label>
              <Select
                value={configValues.output || "stdout"}
                onValueChange={(value: string) => setConfigValues({ ...configValues, output: value })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="stdout">Stdout</SelectItem>
                  <SelectItem value="file">File</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {configValues.output === "file" && (
              <div className="grid gap-2">
                <Label htmlFor="log-path">Log File Path</Label>
                <Input
                  id="log-path"
                  placeholder="/var/log/olb/access.log"
                  value={configValues.filePath || ""}
                  onChange={(e) => setConfigValues({ ...configValues, filePath: e.target.value })}
                />
              </div>
            )}
          </>
        )
      case "api_key":
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="apikey-header">API Key Header</Label>
              <Input
                id="apikey-header"
                value={configValues.header || "X-API-Key"}
                onChange={(e) => setConfigValues({ ...configValues, header: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="apikey-keys">Valid API Keys (one per line)</Label>
              <Textarea
                id="apikey-keys"
                placeholder="key1&#10;key2&#10;key3"
                rows={4}
                value={(configValues.keys || []).join("\n")}
                onChange={(e) => setConfigValues({ ...configValues, keys: e.target.value.split("\n").filter(k => k.trim()) })}
              />
            </div>
          </>
        )
      case "transform":
        return (
          <>
            <div className="grid gap-2">
              <Label htmlFor="transform-headers">Add/Modify Headers (JSON)</Label>
              <Textarea
                id="transform-headers"
                placeholder='{"X-Custom-Header": "value"}'
                rows={3}
                value={JSON.stringify(configValues.headers || {}, null, 2)}
                onChange={(e) => {
                  try {
                    setConfigValues({ ...configValues, headers: JSON.parse(e.target.value) })
                  } catch {}
                }}
              />
            </div>
          </>
        )
      default:
        return <p className="text-muted-foreground">No configuration available for this middleware.</p>
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Middleware</h1>
          <p className="text-muted-foreground">Configure request/response middleware chain</p>
        </div>
        <Button onClick={() => toast.success("Configuration saved")}>
          Save Changes
        </Button>
      </div>

      <div className="flex gap-2">
        {categories.map(cat => (
          <Button
            key={cat.id}
            variant={selectedCategory === cat.id ? "default" : "outline"}
            size="sm"
            onClick={() => setSelectedCategory(cat.id)}
          >
            {cat.label}
          </Button>
        ))}
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {filteredMiddleware.map((middleware) => (
          <Card key={middleware.id} className={cn(
            "transition-colors",
            middleware.enabled ? "border-primary/50" : ""
          )}>
            <CardHeader className="pb-3">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className={cn(
                    "p-2 rounded-lg",
                    middleware.enabled ? "bg-primary/10" : "bg-muted"
                  )}>
                    <middleware.icon className={cn(
                      "h-5 w-5",
                      middleware.enabled ? "text-primary" : "text-muted-foreground"
                    )} />
                  </div>
                  <div>
                    <CardTitle className="text-base">{middleware.name}</CardTitle>
                    <Badge variant="outline" className={cn("text-xs capitalize mt-1", categoryColors[middleware.category])}>
                      {middleware.category}
                    </Badge>
                  </div>
                </div>
                <Switch
                  checked={middleware.enabled}
                  onCheckedChange={() => toggleMiddleware(middleware.id)}
                />
              </div>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground mb-4">{middleware.description}</p>
              <Button
                variant="outline"
                size="sm"
                className="w-full"
                disabled={!middleware.enabled}
                onClick={() => openConfigDialog(middleware)}
              >
                <Settings2 className="mr-2 h-4 w-4" />
                Configure
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>

      <Dialog open={configDialogOpen} onOpenChange={setConfigDialogOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {selectedMiddleware && <selectedMiddleware.icon className="h-5 w-5" />}
              {selectedMiddleware?.name} Configuration
            </DialogTitle>
            <DialogDescription>
              {selectedMiddleware?.description}
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            {renderConfigFields()}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfigDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={saveConfig}>
              Save Configuration
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
