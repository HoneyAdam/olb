import { useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Shield, Plus, Trash2, CheckCircle, AlertCircle, Upload } from "lucide-react"
import { toast } from "sonner"

interface Certificate {
  id: string
  domain: string
  issuer: string
  notBefore: string
  notAfter: string
  daysUntilExpiry: number
  autoRenew: boolean
  source?: 'manual' | 'acme'
}

const mockCertificates: Certificate[] = [
  {
    id: "1",
    domain: "*.openloadbalancer.dev",
    issuer: "Let's Encrypt R3",
    notBefore: "2025-01-01",
    notAfter: "2025-04-01",
    daysUntilExpiry: 45,
    autoRenew: true,
    source: 'acme'
  },
  {
    id: "2",
    domain: "admin.openloadbalancer.dev",
    issuer: "Let's Encrypt R3",
    notBefore: "2025-01-15",
    notAfter: "2025-04-15",
    daysUntilExpiry: 60,
    autoRenew: true,
    source: 'acme'
  }
]

export function CertificatesPage() {
  const [certs, setCerts] = useState<Certificate[]>(mockCertificates)

  // Add Certificate Dialog State
  const [certDialogOpen, setCertDialogOpen] = useState(false)
  const [certSource, setCertSource] = useState<'manual' | 'acme'>('acme')
  const [newCert, setNewCert] = useState({
    domain: "",
    email: "",
    certContent: "",
    keyContent: "",
    autoRenew: true,
  })

  const getExpiryBg = (days: number) => {
    if (days < 7) return "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300"
    if (days < 30) return "bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300"
    return "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
  }

  const handleAddCertificate = () => {
    const cert: Certificate = {
      id: Math.random().toString(36).substr(2, 9),
      domain: newCert.domain,
      issuer: certSource === 'acme' ? "Let's Encrypt" : "Manual",
      notBefore: new Date().toISOString().split('T')[0],
      notAfter: new Date(Date.now() + 90 * 24 * 60 * 60 * 1000).toISOString().split('T')[0],
      daysUntilExpiry: 90,
      autoRenew: newCert.autoRenew,
      source: certSource,
    }
    setCerts([...certs, cert])
    setCertDialogOpen(false)
    setNewCert({ domain: "", email: "", certContent: "", keyContent: "", autoRenew: true })
    toast.success(`Certificate for "${cert.domain}" added successfully`)
  }

  const handleDeleteCert = (id: string) => {
    setCerts(certs.filter(c => c.id !== id))
    toast.success("Certificate deleted successfully")
  }

  const handleRenewCert = (id: string) => {
    setCerts(certs.map(c => c.id === id ? { ...c, daysUntilExpiry: 90, notAfter: new Date(Date.now() + 90 * 24 * 60 * 60 * 1000).toISOString().split('T')[0] } : c))
    toast.success("Certificate renewal initiated")
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">TLS Certificates</h1>
          <p className="text-muted-foreground">Manage SSL/TLS certificates</p>
        </div>
        <Dialog open={certDialogOpen} onOpenChange={setCertDialogOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              Add Certificate
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[600px]">
            <DialogHeader>
              <DialogTitle>Add Certificate</DialogTitle>
              <DialogDescription>
                Add a new TLS certificate manually or via ACME/Let's Encrypt.
              </DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="flex gap-2">
                <Button
                  variant={certSource === 'acme' ? 'default' : 'outline'}
                  className="flex-1"
                  onClick={() => setCertSource('acme')}
                >
                  Let's Encrypt
                </Button>
                <Button
                  variant={certSource === 'manual' ? 'default' : 'outline'}
                  className="flex-1"
                  onClick={() => setCertSource('manual')}
                >
                  Manual Upload
                </Button>
              </div>

              {certSource === 'acme' ? (
                <>
                  <div className="grid gap-2">
                    <Label htmlFor="domain">Domain</Label>
                    <Input
                      id="domain"
                      placeholder="e.g., *.example.com"
                      value={newCert.domain}
                      onChange={(e) => setNewCert({ ...newCert, domain: e.target.value })}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="email">Email</Label>
                    <Input
                      id="email"
                      type="email"
                      placeholder="admin@example.com"
                      value={newCert.email}
                      onChange={(e) => setNewCert({ ...newCert, email: e.target.value })}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <Label htmlFor="auto-renew">Auto-renewal</Label>
                    <Switch
                      id="auto-renew"
                      checked={newCert.autoRenew}
                      onCheckedChange={(checked) => setNewCert({ ...newCert, autoRenew: checked })}
                    />
                  </div>
                </>
              ) : (
                <>
                  <div className="grid gap-2">
                    <Label htmlFor="cert-domain">Domain</Label>
                    <Input
                      id="cert-domain"
                      placeholder="e.g., example.com"
                      value={newCert.domain}
                      onChange={(e) => setNewCert({ ...newCert, domain: e.target.value })}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="cert-content">Certificate (PEM)</Label>
                    <Textarea
                      id="cert-content"
                      placeholder="-----BEGIN CERTIFICATE-----"
                      rows={4}
                      value={newCert.certContent}
                      onChange={(e) => setNewCert({ ...newCert, certContent: e.target.value })}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="key-content">Private Key (PEM)</Label>
                    <Textarea
                      id="key-content"
                      placeholder="-----BEGIN PRIVATE KEY-----"
                      rows={4}
                      value={newCert.keyContent}
                      onChange={(e) => setNewCert({ ...newCert, keyContent: e.target.value })}
                    />
                  </div>
                </>
              )}
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setCertDialogOpen(false)}>
                Cancel
              </Button>
              <Button
                onClick={handleAddCertificate}
                disabled={certSource === 'acme' ? !newCert.domain || !newCert.email : !newCert.domain || !newCert.certContent || !newCert.keyContent}
              >
                Add Certificate
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Total Certificates</CardTitle>
            <Shield className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{certs.length}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Auto-Renewal</CardTitle>
            <CheckCircle className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {certs.filter(c => c.autoRenew).length}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium">Expiring Soon</CardTitle>
            <AlertCircle className="h-4 w-4 text-amber-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {certs.filter(c => c.daysUntilExpiry < 30).length}
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="space-y-4">
        {certs.map((cert) => (
          <Card key={cert.id}>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Shield className="h-5 w-5 text-primary" />
                  <div>
                    <CardTitle className="text-base">{cert.domain}</CardTitle>
                    <CardDescription>{cert.issuer}</CardDescription>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Badge className={getExpiryBg(cert.daysUntilExpiry)}>
                    {cert.daysUntilExpiry} days
                  </Badge>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleRenewCert(cert.id)}
                  >
                    <Upload className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-destructive"
                    onClick={() => handleDeleteCert(cert.id)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <span className="text-muted-foreground">Valid From:</span>
                  <span className="ml-2">{cert.notBefore}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Valid Until:</span>
                  <span className="ml-2">{cert.notAfter}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Source:</span>
                  <span className="ml-2 capitalize">{cert.source}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">Auto-renew:</span>
                  <span className="ml-2">{cert.autoRenew ? 'Yes' : 'No'}</span>
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
