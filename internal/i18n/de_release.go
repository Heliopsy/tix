// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

func init() {
	register(German, map[string]string{
		"Update available": "Update verfügbar",
		"v%s is available": "v%s ist verfügbar",
		"This installation runs v%s. The commands below install the newer release.": "Diese Installation läuft mit v%s. Die folgenden Befehle installieren die neuere Version.",
		"Up to date":                              "Aktuell",
		"This installation is up to date":         "Diese Installation ist aktuell",
		"v%s is the newest published release.":    "v%s ist die neueste veröffentlichte Version.",
		"Not comparable":                          "Nicht vergleichbar",
		"This build's version cannot be compared": "Die Version dieses Builds lässt sich nicht vergleichen",
		"v%s is the newest published release, and v%s is not a version number it can be ordered against.": "v%s ist die neueste veröffentlichte Version, und v%s ist keine Versionsnummer, mit der sie sich vergleichen lässt.",
		"Check failed":                          "Prüfung fehlgeschlagen",
		"The release feed could not be reached": "Der Release-Feed war nicht erreichbar",
		"That is the ordinary case for an isolated network. The server tries again within six hours.": "Das ist in einem isolierten Netzwerk der Normalfall. Der Server versucht es innerhalb von sechs Stunden erneut.",
		"Not checked":     "Nicht geprüft",
		"Not checked yet": "Noch nicht geprüft",
		"The server asks the release feed in the background. Reload in a moment.": "Der Server fragt den Release-Feed im Hintergrund ab. Laden Sie die Seite gleich neu.",
		"How to upgrade": "So aktualisieren Sie",
		"Use the way this server was installed. Nothing here restarts a running tix serve: restart it once the new version is in place.": "Verwenden Sie den Weg, auf dem dieser Server installiert wurde. Nichts hier startet ein laufendes tix serve neu: Starten Sie es neu, sobald die neue Version bereitliegt.",
		"Release binary": "Release-Binary",
		"Installed with install.sh or unpacked from a release archive. Checks the download against its published checksum before replacing the binary.": "Mit install.sh installiert oder aus einem Release-Archiv entpackt. Prüft den Download gegen die veröffentlichte Prüfsumme, bevor das Binary ersetzt wird.",
		"Container image": "Container-Image",
		"Pull the new tag, then recreate the container on the same volume.": "Ziehen Sie das neue Tag und erstellen Sie den Container auf demselben Volume neu.",
		"Go toolchain": "Go-Toolchain",
		"Installed with go install. tix update refuses this copy, because the toolchain owns it.": "Mit go install installiert. tix update lehnt diese Kopie ab, weil sie der Toolchain gehört.",
		"Installed from a package manager? Upgrade it the way you installed it.":                  "Aus einem Paketmanager installiert? Aktualisieren Sie auf dem Weg, auf dem Sie installiert haben.",
		"What changed in v%s": "Was sich in v%s geändert hat",
		"What's new":          "Neuigkeiten",
		"From the changelog this build carries: the version running here, then the releases before it.": "Aus dem Changelog dieses Builds: die hier laufende Version, dann die Versionen davor.",
		"You are running this": "Diese Version läuft hier",
		"v%s is not a tagged release, so these are the newest entries in the changelog it was built with.": "v%s ist keine getaggte Version, daher sind dies die neuesten Einträge des Changelogs, mit dem sie gebaut wurde.",
		"The changelog this build carries has no entry for v%s, so these are its newest entries.":          "Der Changelog dieses Builds hat keinen Eintrag für v%s, daher sind dies seine neuesten Einträge.",
		"This build carries no changelog.": "Dieser Build enthält keinen Changelog.",
		"Links have been removed.":         "Links wurden entfernt.",
	})
}
