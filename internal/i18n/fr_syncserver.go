// SPDX-License-Identifier: AGPL-3.0-or-later

package i18n

// The external sync screen's account of what runs on the server: the stored
// mapping document, the sealed credentials and what a source still lacks.
func init() {
	register(French, map[string]string{ // #nosec G101 -- interface text keyed by its English, not credentials
		"Sync runs on the tix server, not on your machine.": "La synchronisation s’exécute sur le serveur tix, pas sur votre machine.",
		"The mapping and the credentials a run uses have to be on the server. Upload the mapping and store the credentials in the source dialog, or from the command line with tix sync source set; the server's operator can use TIX_SYNC_ environment variables instead, for the sources it binds to them. Each source below says what its runs will use and what the server is still missing.": "Le mapping et les identifiants utilisés par une exécution doivent se trouver sur le serveur. Téléversez le mapping et enregistrez les identifiants dans la boîte de dialogue de la source, ou en ligne de commande avec tix sync source set ; l’opérateur du serveur peut utiliser à la place les variables d’environnement TIX_SYNC_, pour les sources qu’il y associe. Chaque source ci-dessous indique ce que ses exécutions utiliseront et ce qui manque encore au serveur.",
		"On the server":        "Sur le serveur",
		"Mapping":              "Mapping",
		"Credentials":          "Identifiants",
		"stored on the source": "enregistrés sur la source",
		"file on the server":   "fichier sur le serveur",
		"from the environment": "depuis l’environnement",
		"none needed":          "aucun requis",
		"missing":              "manquant",
		"Missing on the server, so it cannot run:": "Manquant sur le serveur, elle ne peut donc pas s’exécuter :",
		"Set up": "Configurer",
		"Runs execute on the tix server. Everything filled in here is stored there, on the source: the mapping document, the credentials and the settings. Anything left empty comes from the server's TIX_SYNC_ environment variables, which the Which variables list on each row names.": "Les exécutions ont lieu sur le serveur tix. Tout ce qui est rempli ici y est enregistré, sur la source : le document de mapping, les identifiants et les réglages. Ce qui reste vide provient des variables d’environnement TIX_SYNC_ du serveur, que la liste Quelles variables de chaque ligne nomme.",
		"Setting up %s. Saving keeps its stored mapping and credentials unless you replace or clear them.": "Configuration de %s. L’enregistrement conserve son mapping et ses identifiants enregistrés, sauf si vous les remplacez ou les effacez.",
		"Checked on the server, nothing saved yet":                                                         "Vérifié sur le serveur, rien d’enregistré pour l’instant",
		"Imports into":             "Importe dans",
		"Identified by":            "Identifiés par",
		"Statuses":                 "Statuts",
		"Missing on the server":    "Manquant sur le serveur",
		"nothing, a run can start": "rien, une exécution peut démarrer",
		"Credentials typed before a check are not kept: type them again when you save.": "Les identifiants saisis avant une vérification ne sont pas conservés : saisissez-les à nouveau lors de l’enregistrement.",
		"Mapping document": "Document de mapping",
		"The YAML that says which tix project records land in and what each external field becomes. Paste it here or choose the file below; it is stored on the source, on the server. Use Check to validate it against the project before saving.": "Le YAML qui indique dans quel projet tix les enregistrements arrivent et ce que devient chaque champ externe. Collez-le ici ou choisissez le fichier ci-dessous ; il est enregistré sur la source, sur le serveur. Utilisez Vérifier pour le valider par rapport au projet avant d’enregistrer.",
		"Or upload a mapping file":   "Ou téléversez un fichier de mapping",
		"Mapping file on the server": "Fichier de mapping sur le serveur",
		"A file in this tenant's sync directory on the server, which its operator sets with sync.file_root; a path outside it is refused. A document pasted or uploaded above wins over it.": "Un fichier du répertoire de synchronisation de cet espace sur le serveur, que son opérateur définit avec sync.file_root ; un chemin en dehors est refusé. Un document collé ou téléversé ci-dessus l’emporte sur lui.",
		"Remove the stored mapping document": "Retirer le document de mapping enregistré",
		"Stored on the server, sealed with its key, and never shown again: only which ones are set and the last four characters of a long one. Leave these empty to keep what is stored. OpenProject takes the user apikey and an API key as the password; Jira takes an account and an API token.": "Enregistrés sur le serveur, scellés avec sa clé, et plus jamais affichés : seulement lesquels sont définis et les quatre derniers caractères d’un long. Laissez ces champs vides pour garder ce qui est enregistré. OpenProject prend l’utilisateur apikey et une clé d’API comme mot de passe ; Jira prend un compte et un jeton d’API.",
		"API token":                     "Jeton d’API",
		"User":                          "Utilisateur",
		"Password or API key":           "Mot de passe ou clé d’API",
		"Remove the stored credentials": "Retirer les identifiants enregistrés",
		"Each setting filled in is stored on the source and wins over its environment variable. generic takes a file in this tenant's sync directory on the server; jira and openproject take a url, a project and a query. The server's environment credentials are never sent to a url stored here.": "Chaque réglage rempli est enregistré sur la source et l’emporte sur sa variable d’environnement. generic prend un fichier du répertoire de synchronisation de cet espace sur le serveur ; jira et openproject prennent une url, un projet et une requête. Les identifiants d’environnement du serveur ne sont jamais envoyés à une url enregistrée ici.",
		"File on the server": "Fichier sur le serveur",
		"Check":              "Vérifier",
		"A name, a system, and what the form stores on it: the settings, the mapping document and the credentials. A stored value wins over its environment variable on the server, and one left empty is read from that variable when a run starts. Credentials are sealed with the server's key and never shown, audited or logged.": "Un nom, un système, et ce que le formulaire y enregistre : les réglages, le document de mapping et les identifiants. Une valeur enregistrée l’emporte sur sa variable d’environnement sur le serveur, et une valeur laissée vide est lue depuis cette variable au démarrage d’une exécution. Les identifiants sont scellés avec la clé du serveur et ne sont jamais affichés, audités ni journalisés.",
		"A mapping":                          "Un mapping",
		"Start from the OpenProject mapping": "Partir du mapping OpenProject",
		"fills this in with a mapping for OpenProject work packages: subject, description, status, priority, assignee, dates, parent and type. Replace CHANGE-ME with the key of the tix project to import into.": "remplit ce champ avec un mapping pour les work packages OpenProject : sujet, description, statut, priorité, responsable, dates, parent et type. Remplacez CHANGE-ME par la clé du projet tix dans lequel importer.",
		"People with no tix account": "Personnes sans compte tix",
		"Map them under users in the mapping, or set users.unknown to placeholder.": "Associez-les sous users dans le mapping, ou définissez users.unknown sur placeholder.",
	})
}
