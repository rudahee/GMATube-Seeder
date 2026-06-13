package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sys/windows/registry"
)

const (
	appName        = "GMATube Seeder Daemon"
	configFileName = "config.json"
)

type Config struct {
	CPUCores         int  `json:"cpu_cores"`
	MaxRAMMB         int  `json:"max_ram_mb"`
	MaxDiskGB        int  `json:"max_disk_gb"`
	DownloadKBps     int  `json:"download_kbps"`
	UploadKBps       int  `json:"upload_kbps"`
	StartWithWindows bool `json:"start_with_windows"`
}

type FakeStats struct {
	ActiveTorrents int
	DownloadKBps   int
	UploadKBps     int
	SharedGB       float64
	ConnectedPeers int
}

func main() {
	cfg := loadConfig()
	runtime.GOMAXPROCS(cfg.CPUCores)

	daemon := NewSeederDaemon(cfg)
	daemon.Start()

	guiApp := app.New()
	window := guiApp.NewWindow(appName)
	window.Resize(fyne.NewSize(460, 520))

	cpuEntry := widget.NewEntry()
	cpuEntry.SetText(strconv.Itoa(cfg.CPUCores))

	ramEntry := widget.NewEntry()
	ramEntry.SetText(strconv.Itoa(cfg.MaxRAMMB))

	diskEntry := widget.NewEntry()
	diskEntry.SetText(strconv.Itoa(cfg.MaxDiskGB))

	downloadEntry := widget.NewEntry()
	downloadEntry.SetText(strconv.Itoa(cfg.DownloadKBps))

	uploadEntry := widget.NewEntry()
	uploadEntry.SetText(strconv.Itoa(cfg.UploadKBps))

	startupCheck := widget.NewCheck("Iniciar automáticamente con Windows", nil)
	startupCheck.SetChecked(cfg.StartWithWindows)

	statusLabel := widget.NewLabel("Estado: daemon activo")
	activeTorrentsLabel := widget.NewLabel("Torrents activos: 0")
	downloadLabel := widget.NewLabel("Descarga actual: 0 KB/s")
	uploadLabel := widget.NewLabel("Subida actual: 0 KB/s")
	sharedLabel := widget.NewLabel("Compartido: 0.00 GB")
	peersLabel := widget.NewLabel("Peers conectados: 0")

	saveButton := widget.NewButton("Guardar configuración", func() {
		newCfg := Config{
			CPUCores:         parseIntOrDefault(cpuEntry.Text, cfg.CPUCores),
			MaxRAMMB:         parseIntOrDefault(ramEntry.Text, cfg.MaxRAMMB),
			MaxDiskGB:        parseIntOrDefault(diskEntry.Text, cfg.MaxDiskGB),
			DownloadKBps:     parseIntOrDefault(downloadEntry.Text, cfg.DownloadKBps),
			UploadKBps:       parseIntOrDefault(uploadEntry.Text, cfg.UploadKBps),
			StartWithWindows: startupCheck.Checked,
		}

		if newCfg.CPUCores < 1 {
			newCfg.CPUCores = 1
		}

		if newCfg.CPUCores > runtime.NumCPU() {
			newCfg.CPUCores = runtime.NumCPU()
		}

		cfg = newCfg
		runtime.GOMAXPROCS(cfg.CPUCores)
		daemon.UpdateConfig(cfg)

		if err := saveConfig(cfg); err != nil {
			statusLabel.SetText(fmt.Sprintf("Error guardando configuración: %v", err))
			return
		}

		if err := setWindowsAutostart(cfg.StartWithWindows); err != nil {
			statusLabel.SetText(fmt.Sprintf("Error configurando inicio automático: %v", err))
			return
		}

		statusLabel.SetText("Configuración guardada correctamente")
	})

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			stats := daemon.Stats()

			activeTorrentsLabel.SetText(fmt.Sprintf("Torrents activos: %d", stats.ActiveTorrents))
			downloadLabel.SetText(fmt.Sprintf("Descarga actual: %d KB/s", stats.DownloadKBps))
			uploadLabel.SetText(fmt.Sprintf("Subida actual: %d KB/s", stats.UploadKBps))
			sharedLabel.SetText(fmt.Sprintf("Compartido: %.2f GB", stats.SharedGB))
			peersLabel.SetText(fmt.Sprintf("Peers conectados: %d", stats.ConnectedPeers))
		}
	}()

	window.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("GMATube Seeder Daemon", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),

		widget.NewSeparator(),

		widget.NewLabel("Configuración de recursos"),

		widget.NewLabel(fmt.Sprintf("Núcleos de CPU a usar. Disponibles: %d", runtime.NumCPU())),
		cpuEntry,

		widget.NewLabel("RAM máxima en MB"),
		ramEntry,

		widget.NewLabel("Espacio máximo en disco en GB"),
		diskEntry,

		widget.NewLabel("Límite de descarga en KB/s"),
		downloadEntry,

		widget.NewLabel("Límite de subida en KB/s"),
		uploadEntry,

		startupCheck,

		saveButton,

		widget.NewSeparator(),

		widget.NewLabel("Stats actuales"),
		statusLabel,
		activeTorrentsLabel,
		downloadLabel,
		uploadLabel,
		sharedLabel,
		peersLabel,
	))

	window.SetCloseIntercept(func() {
		window.Hide()
	})

	window.ShowAndRun()
}

type SeederDaemon struct {
	config Config
	stats  FakeStats
}

func NewSeederDaemon(config Config) *SeederDaemon {
	return &SeederDaemon{
		config: config,
		stats: FakeStats{
			ActiveTorrents: 0,
			DownloadKBps:   0,
			UploadKBps:     0,
			SharedGB:       0,
			ConnectedPeers: 0,
		},
	}
}

func (d *SeederDaemon) Start() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			d.generateFakeStats()
		}
	}()
}

func (d *SeederDaemon) UpdateConfig(config Config) {
	d.config = config
}

func (d *SeederDaemon) Stats() FakeStats {
	return d.stats
}

func (d *SeederDaemon) generateFakeStats() {
	maxDownload := d.config.DownloadKBps
	maxUpload := d.config.UploadKBps

	if maxDownload <= 0 {
		maxDownload = 1
	}

	if maxUpload <= 0 {
		maxUpload = 1
	}

	d.stats.ActiveTorrents = rand.Intn(5)
	d.stats.DownloadKBps = rand.Intn(maxDownload)
	d.stats.UploadKBps = rand.Intn(maxUpload)
	d.stats.ConnectedPeers = rand.Intn(80)
	d.stats.SharedGB += float64(d.stats.UploadKBps) / 1024 / 1024
}

func loadConfig() Config {
	defaultConfig := Config{
		CPUCores:         max(1, runtime.NumCPU()/2),
		MaxRAMMB:         1024,
		MaxDiskGB:        20,
		DownloadKBps:     2048,
		UploadKBps:       1024,
		StartWithWindows: false,
	}

	path := configPath()

	data, err := os.ReadFile(path)
	if err != nil {
		_ = saveConfig(defaultConfig)
		return defaultConfig
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig
	}

	if cfg.CPUCores < 1 {
		cfg.CPUCores = 1
	}

	if cfg.CPUCores > runtime.NumCPU() {
		cfg.CPUCores = runtime.NumCPU()
	}

	return cfg
}

func saveConfig(config Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath(), data, 0644)
}

func configPath() string {
	exe, err := os.Executable()
	if err != nil {
		return configFileName
	}

	return filepath.Join(filepath.Dir(exe), configFileName)
}

func parseIntOrDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func setWindowsAutostart(enabled bool) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.SET_VALUE|registry.QUERY_VALUE,
	)
	if err != nil {
		return err
	}
	defer key.Close()

	if enabled {
		return key.SetStringValue(appName, exePath)
	}

	err = key.DeleteValue(appName)
	if err != nil && err != registry.ErrNotExist {
		return err
	}

	return nil
}
