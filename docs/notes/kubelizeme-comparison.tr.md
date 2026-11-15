# Notlar: KubeBench ve KubelizeMe

*English: [kubelizeme-comparison.md](kubelizeme-comparison.md)*

Bu notlar KubeBench'in, v0.5'i planlarken referans aldığımız masaüstü
Kubernetes IDE'si **KubelizeMe**'den nerelerde ayrıştığını kayıt altına
alır. Hangi fikirleri benimsediğimizi, hangilerini bilerek farklı
yaptığımızı ve neyin açık kaldığını anlatır.

> **Yöntem.** Elimizde yalnızca KubelizeMe'nin derlenmiş frontend paketi
> vardı (Tauri uygulaması; sıkıştırılmış Vite parçaları, kaynak kod yok).
> Özellik envanteri yaklaşık 220 Tauri komut adından (`list_pods`,
> `helm_upgrade_dry_run`, `run_security_audit`, …) ve parçalardaki arayüz
> metinlerinden çıkarıldı. Davranış ayrıntıları bu yüzden çıkarıma dayanır.
> KubeBench'te KubelizeMe'ye ait hiçbir kod, görsel ya da metin
> kullanılmamıştır.

> **Durum.** Aşağıdaki v0.5 sütunu 0.5.0'da gerçekten çıkanlarla
> karşılaştırılarak güncellendi (kod üzerinde 2026-11-15'te denetlendi).
> Hâlâ eksik olanlar §4'te listelenir.

## 1. Konumlandırma

| | KubelizeMe | KubeBench |
|---|---|---|
| Biçim | Masaüstü uygulaması (Tauri); her makinede tek kullanıcı | Web uygulaması: arayüzü gömülü tek bir Go binary'si. Cluster içinde, VM'de ya da yerelde çalışır |
| Kimlik bilgileri | Yerel kubeconfig dosyaları, context'ler, servis hesabı token'ları, WSL kubeconfig'leri | Sunucu tarafında tutulur: cluster içi ServiceAccount, yüklenen kubeconfig'ler ya da sunucudaki `KUBECONFIG` |
| Kullanıcılar | Tek kullanıcı | Çok kullanıcılı: oturumlar, global roller, cluster bazında roller (izleyici/operatör), salt okunur cluster'lar, denetim kaydı *(v0.5)* |
| Aynı anda açık cluster | Birden çok context, renk grupları | Kullanıcı başına bir aktif cluster; geçişte arayüz yeniden yüklenir |
| Yerel araçlar | `helm`/`kubectl` çağırır; araç denetimi ve binary yolu ayarları var | Yalnızca yerel SDK'lar (client-go, Helm v3 SDK); sunucuda ek bir araç gerekmez |
| Dağıtım | Masaüstü kurulum paketleri + otomatik güncelleyici | Container imajı, Helm chart'ı, düz manifest'ler, `.deb`/`.rpm`, tarball'lar, `curl \| bash` kurulumu, offline paket; yeni sürümleri ve yükseltme adımlarını gösteren ama hiçbir şey kurmayan güncelleme denetimi *(v0.5)* |
| Genişletilebilirlik | Kapalı uygulama; AI tarafını MCP sunucularıyla genişletir | Eklenti platformu: yerleşik backend+frontend eklentileri, yönetici tarafından açma/kapama, çalışma anında yüklenen harici arayüz eklentileri *(v0.5)* |
| Diller | İngilizce | İngilizce, Türkçe ve Almanca, eklenti bazında çeviri namespace'leri; yeni diller yalnızca veri ile eklenir *(v0.5)* |

## 2. Özellik karşılaştırması

Gösterim: ✓ var · ◐ kısmen · — yok · **v0.5** bu sürümde eklendi.

### Kaynaklar ve düzenleme

| Alan | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Yerleşik kaynak türleri (iş yükleri, config, ağ, depolama, RBAC, webhook'lar, kotalar, HPA/PDB, …) | ✓ | ✓ | ✓ |
| CRD'ler ve özel kaynaklar | ✓ | ✓ | ✓ |
| Gateway API / Traefik rotaları | ✓ (`list_network_routes`) | CRD tarayıcısıyla | CRD tarayıcısıyla |
| YAML görünümü | ✓ | ✓ (salt okunur) | ✓ |
| Düzenle + uygula, sunucu tarafı dry-run, canlı nesneyle diff | ✓ | — | **v0.5** |
| "Neat" YAML (sunucunun yönettiği alanları ayıklama) | ✓ | — | **v0.5** |
| YAML'dan oluşturma / dosya içe aktarma | ✓ (Composer, içe aktarma) | — | **v0.5** |
| Alt kaynaklara etkisini açıklayan genel silme, çakışma algılama | ✓ | yalnızca Pod'lar | **v0.5** |
| Label/annotation düzenleme | ✓ | — | **v0.5** |
| Secret değerlerini gösterme (onayla) | ✓ | — (her zaman gizli) | **v0.5** (exec yetkisiyle, denetim kaydına yazılır) |

### Operasyonlar

| Alan | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Deployment ölçekleme/yeniden başlatma | ✓ | ✓ | ✓ |
| StatefulSet, DaemonSet yeniden başlatma/ölçekleme | ✓ | — | **v0.5** |
| Rollout geçmişi, geri alma, duraklatma/sürdürme | ✓ | — | **v0.5** |
| Node cordon / drain / taint | ✓ | — | **v0.5** |
| Zorla silme, toplu Pod silme | ✓ | — | **v0.5** |
| Debug (ephemeral) container | ✓ | — | **v0.5** |
| CronJob'u hemen tetikleme / askıya alma | — (görülmedi) | — | **v0.5** |
| Namespace oluşturma/silme | ✓ | — | **v0.5** |
| Port-forward | ✓ (yerel portlar) | — | — (yol haritası: web uygulamasına backend HTTP proxy'si yerel porttan daha uygun) |

### Terminal ve loglar

| Alan | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Etkileşimli kabuk (PTY, yeniden boyutlandırma, çoklu oturum) | ✓ (xterm, arama, prompt işaretleri) | ◐ tek seferlik `sh -c`, 20 sn sınır | **v0.5** (WebSocket PTY + xterm.js) |
| Cluster kubeconfig'iyle yerel terminal | ✓ | — | — (web uygulamasında yerel kabuk yok; tasarım gereği) |
| Canlı log akışı: takip, tail boyutu, önceki container, zaman damgaları | ✓ | ◐ 300 satırlık anlık görüntü | **v0.5** |
| Label seçiciyle çoklu Pod logu | ✓ | — | **v0.5** |
| Logları dosyaya kaydetme | ✓ | — | **v0.5** (indirme) |

### Helm ve GitOps

| Alan | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Release'ler, geçmiş, değerler, geri alma, kaldırma | ✓ | ✓ | ✓ |
| Dry-run ile kurma / yükseltme | ✓ | — | **v0.5** |
| Depo ekleme/silme/güncelleme, chart arama | ✓ (yerel `helm` CLI) | — | **v0.5** (SDK; depo bilgileri sunucuda) |
| Revizyonlar arası values diff'i | ✓ | — | **v0.5** |
| Yüklenen chart'tan kurulum (offline) | — | — | **v0.5** |
| Argo CD uygulamaları, sync, refresh, geri alma | — | — | **v0.5** (Argo CD eklentisi; CRD'ler üzerinden çalışır) |

### Depolama

| Alan | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| PV / PVC / StorageClass listeleri | ✓ | ✓ | ✓ |
| Doğrulama + dry-run'lı PVC / PV / StorageClass sihirbazları | ◐ (Composer içinde) | — | **v0.5** (Storage eklentisi) |
| PVC büyütme | — | — | **v0.5** |

### Güvenlik ve erişim

| Alan | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Güvenlik duruşu: CIS (5. bölüm), NSA/CISA, Pod Security Standards | ✓ | — | **v0.5** (Security eklentisi; CIS 5. bölüm + PSS eşlemesi) |
| Kabul edilen riskler | ✓ cluster içi bir CRD'de ("findings ledger") | — | **v0.5**: KubeBench veritabanında tutulur, cluster'ın tüm kullanıcılarıyla paylaşılır; CRD gerekmez |
| Rapor dışa aktarma (CSV/Markdown/PDF) | ✓ | — | **v0.5** (CSV/Markdown) |
| RBAC "kim ne yapabilir", subject yetkileri | ✓ | — | **v0.5** (Access eklentisi; SubjectAccessReview kontrolleri de var) |
| Admission policy / webhook görünümü | ✓ | ◐ (webhook listesi) | ◐ |
| Context bazında güvenlik / salt okunur mod | ✓ (yerel ayar) | — | **v0.5**: cluster başına salt okunur bayrağı ve izleyici/operatör rolleri; sunucuda uygulanır |
| İşlem denetim kaydı | — (tek kullanıcı) | — | **v0.5** |

### Gözlemlenebilirlik

| Alan | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| metrics-server'dan CPU/bellek | ✓ | ✓ | ✓ |
| Prometheus keşfi, geçmiş, ağ ve dosya sistemi serileri | — (yalnızca "metrics-server not available") | ✓ | ✓ |
| Request/limit ile ayrılabilir kapasite karşılaştırması, overcommit işareti | — | ✓ | ✓ |
| Topoloji grafiği (Ingress → Service → iş yükü → Pod) | ✓ (dagre) | — | **v0.5** (Topology eklentisi) |
| Watch ile canlı güncelleme | ✓ (watch önbelleği) | — (elle ya da aralıklı yenileme) | **v0.5** (paylaşılan informer'lar + SSE) |
| Son uyarıları gösteren cluster panosu | ✓ | ◐ (metrik genel bakışı) | **v0.5** (son uyarılar kartı; Warning filtresi ve nedene göre gruplamalı Olaylar görünümü) |

### Çalışma alanı ve kullanım kolaylığı

| Alan | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Komut paleti | ✓ (+ `get`/`describe`, takma adlar) | ✓ | **v0.5** tamamlamalı ve takma adlı kubectl tarzı `get`/`describe`/`logs`/`exec`/`ns`/`ctx` |
| Klavye kısayolları ve tablolarda klavyeyle gezinme | ✓ | ◐ (yalnızca palet) | **v0.5** (ayarlanabilir, `?` kısayol listesi) |
| Namespace sabitleme/gizleme | ✓ | — | **v0.5** |
| Açık/koyu tema, yazı tipleri, arayüz ölçeği | ✓ | yalnızca koyu | ◐ **v0.5** açık/koyu/sistem ve terminal yazı boyutu; arayüz ölçeği yok |
| Çok dilli arayüz | — | — | **v0.5** (en, tr, de) |
| Bulut sağlayıcısı tespiti ve Kubernetes sürüm desteği (EKS, AKS, GKE, …) | ✓ | — | **v0.5** (sağlayıcı rozetleri, destek sonu uyarısı) |
| Bulut CLI'ları olmadan EKS/GKE/AKS oturum açma | — (görülmedi) | — | **v0.5** (süreç içinde, CLI gerekmez; cluster başına kimlik bilgileri ya da sunucunun kimliği) |
| Uygulama içi güncelleme | ✓ (otomatik güncelleyici) | — | **v0.5** güncelleme denetimi + Yenilikler (tasarım gereği hiçbir şey kurmaz) |
| AI asistanı (çoklu sağlayıcı, araç çağrıları, MCP, playbook'lar) | ✓ | — | — (yol haritası) |
| Manifest Composer (form sihirbazları, şablonlar, çoklu cluster'a uygulama) | ✓ | — | ◐ (Storage sihirbazları + YAML şablonları) |
| Cluster'lar arası karşılaştırma | ✓ | — | — (yol haritası) |

## 3. Bilinçli farklar

- **Kimlik bilgileri sunucuda.** Web aracında tarayıcı cluster kimlik
  bilgilerini hiç görmez. Her Kubernetes çağrısı backend'den geçer;
  backend önce KubeBench rollerini, sonra cluster'ın kendi RBAC'ını
  uygular. Salt okunur mod ve roller bu yüzden sunucuda uygulanır,
  arayüzde saklanan bir tercih değildir.
- **CRD yerine paylaşılan durum.** KubelizeMe, kabul edilen riskleri
  birden çok masaüstü paylaşabilsin diye cluster'a bir `SecurityFinding`
  CRD'si kurar. KubeBench'in zaten çok kullanıcılı bir veritabanı var.
  Kabul edilen riskler, Helm depoları ve eklenti ayarları gibi paylaşılan
  durum orada tutulur ve yönetilen cluster'a dokunulmaz.
- **Sunucuda binary bağımlılığı yok.** Helm ve drain işlemleri Go SDK'larıyla
  yapılır. Container distroless kalır; offline kurulum için imaj yeterlidir.
- **Tek parça yerine eklentiler.** Argo CD, Storage, Security, Access,
  Topology ve Helm birer eklentidir. Yönetici bunları kapatabilir.
  Kullanılamadıklarında nedenini söylerler (örneğin "Argo CD CRD'leri
  bulunamadı"). Üçüncü taraflar da fork etmeden arayüz eklentisi
  yazabilir.
- **Dürüst veri.** Canlı mod hiçbir zaman örnek veriye düşmez. Eksik bir
  yetenek, nedeniyle birlikte "kullanılamıyor" olarak gösterilir.

## 4. v0.5'ten sonra hâlâ eksik olanlar (yol haritası)

1. AI asistanı: sunucu tarafında çalışır, araçları varsayılan olarak salt okunurdur, her işlem denetim kaydına yazılır.
2. Manifest Composer: depolama dışındaki kaynaklar için de çoklu kaynak form sihirbazları ve çoklu cluster'a uygulama.
3. Cluster karşılaştırma (neat ile A/B anlık görüntü diff'i).
4. Backend HTTP proxy'si üzerinden port-forward karşılığı.
5. Kullanıcı başına birden çok aktif cluster (yan yana görünümler).
6. Webhook listelerinin ötesinde admission policy görünümleri (ValidatingAdmissionPolicy, Kyverno, Gatekeeper).
7. Güvenlik raporları için NSA/CISA eşlemesi ve PDF dışa aktarma.
8. Arayüz ölçeği ve yazı tipi ayarları.
9. İsteğe bağlı OIDC/SSO girişi ve kullanıcı bazında Kubernetes impersonation.
10. VM güncellemeleri için APT/YUM deposu.
