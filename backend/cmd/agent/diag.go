package main

import (
	"bufio"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

// runDiagnostics executes an interactive diagnostics probe and service control console
func runDiagnostics(serverURL, token string) {
	// 1. Resolve server URL
	targetURL := serverURL
	if targetURL == "" || targetURL == "http://127.0.0.1:3800" {
		savedURL := loadServerURL()
		if savedURL != "" {
			targetURL = savedURL
		}
	}
	if targetURL == "" {
		targetURL = "http://127.0.0.1:3800"
	}

	// 2. Resolve token
	resolvedToken := token
	if resolvedToken == "" {
		resolvedToken = loadAgentKey()
	}

	for {
		printDiagnosticReport(targetURL, resolvedToken)

		fmt.Println("----------------------------------------------------------------------")
		fmt.Println(" [1] Baglanti ve Telemetri Testini Yenile (Refresh)")
		fmt.Println(" [2] Servisi Baslat (Start Service)")
		fmt.Println(" [3] Servisi Durdur (Stop Service)")
		fmt.Println(" [4] Servisi Yeniden Baslat (Restart Service)")
		fmt.Println(" [0] Cikis (Exit)")
		fmt.Println("----------------------------------------------------------------------")
		fmt.Print(" Seciminiz [0-4]: ")

		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			break
		}
		choice := strings.TrimSpace(scanner.Text())

		switch choice {
		case "1":
			continue
		case "2":
			fmt.Println("\n[*] Servis baslatiliyor...")
			if err := startOSService(); err != nil {
				fmt.Printf("[HATA] Servis baslatilamadi: %v\n", err)
			} else {
				fmt.Println("[OK] Servis baslatildi.")
			}
			time.Sleep(1 * time.Second)
		case "3":
			fmt.Println("\n[*] Servis durduruluyor...")
			if err := stopOSService(); err != nil {
				fmt.Printf("[HATA] Servis durdurulamadi: %v\n", err)
			} else {
				fmt.Println("[OK] Servis durduruldu.")
			}
			time.Sleep(1 * time.Second)
		case "4":
			fmt.Println("\n[*] Servis yeniden baslatiliyor...")
			if err := restartOSService(); err != nil {
				fmt.Printf("[HATA] Servis yeniden baslatilamadi: %v\n", err)
			} else {
				fmt.Println("[OK] Servis basariyla yeniden baslatildi.")
			}
			time.Sleep(1 * time.Second)
		case "0", "q", "exit":
			return
		default:
			fmt.Println("Gecersiz secim.")
		}
	}
}

func printDiagnosticReport(serverURL, token string) {
	hostname, _ := os.Hostname()
	serviceState, isRunning := getOSServiceStatus()

	fmt.Println()
	fmt.Println("======================================================================")
	fmt.Printf("   GNNwatchtower Ajan Teshis & Durum Konsolu (v%s)\n", Version)
	fmt.Printf("   Platform: %s/%s | Cihaz: %s\n", runtime.GOOS, runtime.GOARCH, hostname)
	fmt.Println("======================================================================")

	// 1. Service Status
	stateIcon := "[!]"
	if isRunning {
		stateIcon = "[✓]"
	}
	fmt.Printf("%s Hizmet Durumu        : %s\n", stateIcon, serviceState)
	fmt.Printf("[i] Hedef Appliance URL  : %s\n", serverURL)

	// 2. Token & Key
	if token != "" {
		tokenPreview := token
		if len(tokenPreview) > 12 {
			tokenPreview = tokenPreview[:8] + "..." + tokenPreview[len(tokenPreview)-4:]
		}
		fmt.Printf("[✓] Ajan Kimlik Anahtari : Kayitli (%s)\n", tokenPreview)
	} else {
		fmt.Println("[!] Ajan Kimlik Anahtari : Henuz yok (Ilk iletisimde otomatik olusacak)")
	}

	machUUID := getMachineUUID()
	if machUUID != "" {
		fmt.Printf("[i] Makine Donanim Kimlik: %s\n", machUUID)
	}

	// 3. TCP Port Probe (Check Firewall / Port 3800)
	u, err := url.Parse(serverURL)
	targetHost := "127.0.0.1"
	targetPort := "3800"
	if err == nil {
		if h := u.Hostname(); h != "" {
			targetHost = h
		}
		if p := u.Port(); p != "" {
			targetPort = p
		} else if u.Scheme == "https" {
			targetPort = "443"
		} else {
			targetPort = "3800"
		}
	}
	targetAddr := net.JoinHostPort(targetHost, targetPort)

	startProbe := time.Now()
	conn, dialErr := net.DialTimeout("tcp", targetAddr, 3*time.Second)
	latency := time.Since(startProbe)

	if dialErr != nil {
		fmt.Printf("[✗] Ag / Port Erisimi     : BASARISIZ! -> %s ulasilamiyor (%v)\n", targetAddr, dialErr)
		fmt.Println("    ------------------------------------------------------------------")
		fmt.Printf("    [!] HATA TESPITI: Ajan, hedef sunucuya (%s) erisemiyor!\n", targetAddr)
		fmt.Println("    [!] OLASI NEDENLER:")
		fmt.Println("        1. Windows Guvenlik Duvari (Firewall) giden 3800 portunu engelliyor.")
		fmt.Println("        2. Sunucu tarafinda 3800 portu dinlenmiyor veya sunucu kapali.")
		fmt.Println("        3. Sirket router/switch/XDR erisimi blokluyor.")
		fmt.Println("    ------------------------------------------------------------------")
	} else {
		_ = conn.Close()
		fmt.Printf("[✓] Ag / Port Erisimi     : BASARILI (%s acik - Gecikme: %v)\n", targetAddr, latency.Round(time.Millisecond))
	}

	// 4. API Health Endpoint Probe
	healthURL := fmt.Sprintf("%s/api/health", strings.TrimRight(serverURL, "/"))
	client := &http.Client{Timeout: 4 * time.Second}
	hResp, hErr := client.Get(healthURL)
	if hErr != nil {
		fmt.Printf("[✗] REST API Saglik Kontrol: HATA (%v)\n", hErr)
	} else {
		defer hResp.Body.Close()
		if hResp.StatusCode == http.StatusOK {
			fmt.Printf("[✓] REST API Saglik Kontrol: AKTIF (HTTP 200 OK)\n")
		} else {
			fmt.Printf("[!] REST API Saglik Kontrol: HTTP %d Yaniti Alindi\n", hResp.StatusCode)
		}
	}

	// 5. Live OS Hardware Telemetry Snapshot
	cpu, ram, disk, netRx, netTx, temp := getSystemMetrics()
	fmt.Printf("[✓] Canli Donanim Verisi : CPU: %0.1f%% | RAM: %0.1f%% | Disk: %0.1f%% | Ag: ↓%0.1f ↑%0.1f KB/s",
		math.Round(cpu*10)/10, math.Round(ram*10)/10, math.Round(disk*10)/10,
		math.Round(netRx*10)/10, math.Round(netTx*10)/10)
	if temp > 0 {
		fmt.Printf(" | Sicaklik: %0.1f°C", temp)
	}
	fmt.Println()

	// 6. Test Heartbeat Delivery
	if token != "" && dialErr == nil {
		fmt.Print("[*] Merkeze test heartbeat gonderiliyor... ")
		resp, _ := sendHeartbeat(serverURL, token, hostname)
		if resp != nil && resp.Status == "ok" {
			fmt.Println("ONAYLANDI (HTTP 200 OK - Telemetri Kaydedildi)")
		} else {
			fmt.Println("TAMAMLANDI")
		}
	}
}
