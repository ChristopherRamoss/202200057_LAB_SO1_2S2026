package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// cronMarker identifica la linea que este daemon agrega, para poder
// removerla limpiamente sin tocar otros cronjobs del usuario.
const cronMarker = "# proyecto2-so1-cronjob"

// InstallCronjob agrega la entrada de crontab (cada 2 minutos) si no existe.
func InstallCronjob(scriptPath string) error {
	existing, _ := exec.Command("crontab", "-l").Output()
	if strings.Contains(string(existing), cronMarker) {
		return nil // ya estaba instalado
	}

	newLine := fmt.Sprintf("*/2 * * * * %s >> /tmp/proyecto2_cron.log 2>&1 %s\n", scriptPath, cronMarker)
	newCrontab := string(existing) + newLine

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	return cmd.Run()
}

// RemoveCronjob quita la entrada instalada por este daemon (se llama al
// finalizar el servicio, segun lo pide el enunciado).
func RemoveCronjob() error {
	existing, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		return nil // no habia crontab
	}

	lines := strings.Split(string(existing), "\n")
	var filtered []string
	for _, l := range lines {
		if !strings.Contains(l, cronMarker) {
			filtered = append(filtered, l)
		}
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(filtered, "\n"))
	return cmd.Run()
}
