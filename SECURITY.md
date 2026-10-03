**Die Sicherheitsgrenze ist die gültige DSM-Sitzung, nicht der einzelne Tab oder Browser.** Die reguläre SynoWake-API prüft die Anmeldung serverseitig und verlangt Administratorrechte. Mein anonymer Aufruf auf deiner NAS wurde mit **HTTP 403** abgewiesen. Das entspricht Synologys vorgesehenem [Authentifizierungsverfahren](https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html).

Ja, es werden verschiedene Schlüssel mitgeschickt:

- **DSM-Sitzungscookie:** gehört zur Anmeldung.
- **SynoToken:** zusätzlicher DSM-Anfragetoken.
- **X-SYNO-HASH:** zusätzlicher Sitzungsschutz, den DSMs Anfragefunktion übernimmt.
- **X-SynoWake-CSRF:** eigener Schutz für schreibende SynoWake-Aufrufe, gebunden an Benutzer und Sitzungscookie.

Die normale iframe-Adresse enthält **keinen Sitzungsschlüssel**, sondern nur die Paketversion.

Ein weiterer Tab kann dieselbe Anmeldung verwenden. Auch ein Programm außerhalb des Browsers kann grundsätzlich mit gültigen Sitzungsdaten arbeiten; SynoWake erzwingt keine Browserbindung. Gestohlene Sitzungsdaten sind deshalb gefährlich. Welche zusätzliche IP- oder Hashbindung DSM dabei durchsetzt, habe ich nicht durch Wiederholungsversuche geprüft. Das Schließen von SynoWake beendet die DSM-Anmeldung nicht.

**Beim iframe gibt es einen wichtigen Unterschied:** Eine fremde Webseite kann dessen NAS-Inhalte und Schlüssel aufgrund der [Herkunftsprüfung des Browsers](https://developer.mozilla.org/en-US/docs/Web/Security/Defenses/Same-origin_policy) nicht einfach auslesen. SynoWake und DSM selbst teilen jedoch dieselbe Herkunft. Der iframe trennt ihre Darstellung, **nicht ihre Berechtigungen**. Eine eingeschleuste JavaScript-Ausführung in SynoWake könnte deshalb auch DSM-Funktionen mit deinen Rechten aufrufen. Im aktuellen Quellcode habe ich keine konkrete solche Lücke gefunden; Namen und Suchergebnisse werden als Text dargestellt.

Meine Prüfung hat trotzdem Punkte ergeben, die ich härten würde:

1. **Schutz gegen fremde Einbettung fehlt.** Die tatsächlich ausgelieferte SynoWake-Seite setzt weder `Content-Security-Policy` noch `X-Frame-Options`. Falls der Browser Anmeldungscookies in einer fremden Einbettung zulässt, sind täuschend überlagerte Klicks auf echte App-Schaltflächen denkbar: *Clickjacking*. CSRF schützt davor nicht. Empfohlen ist der HTTP-Header `frame-ancestors 'self'`; das eigene DSM-Fenster bleibt damit erlaubt. [MDN-Dokumentation](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy/frame-ancestors)

2. **DSM sollte über HTTPS verwendet werden.** Die App-Seite ist über HTTP auf Port 5000 erreichbar. Bei Nutzung darüber können Netzwerkangreifer Inhalte und Sitzungskontext mitlesen oder verändern. Token und Hash ersetzen keine Transportverschlüsselung. [OWASP-Empfehlung](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html#transport-layer-security)

3. **Lokale Zugriffe und Zeitplanschlüssel lassen sich enger begrenzen.** Der interne Socket ist für lokale NAS-Prozesse zugänglich. Authentifizierung beziehungsweise Zeitplangeheimnis bleiben erforderlich; einen allgemeinen Zugriff ohne Berechtigung habe ich nicht nachgewiesen. Zeitplangeheimnisse stehen außerdem im Aufgabenbefehl und werden derzeit an angemeldete Administratoren ausgeliefert.

**Automatiken sind ausdrücklich sitzungsunabhängig:** Sie laufen ohne Browser mit einem eigenen Zeitplangeheimnis. Zusätzlich werden lokale Herkunft, Aktivierung, Zeitfenster und Wiederholungen geprüft. Dieser Schlüssel erlaubt die hinterlegte Wake-Aktion, keine allgemeine DSM-Nutzung. Ein DSM-Logout widerruft ihn nicht.

Die ausgewählten Sicherheitstests bestehen. Quellcode und anonyme Abfragen ersetzen allerdings keine vollständige Prüfung von DSM, Cookie-Einstellungen und Sitzungsablauf.

Die [ausführliche Sicherheitsanalyse](C:/Users/Besitzer/Documents/Codex/2026-10-02/ein-paket-f-r-synology-dsm-2/outputs/Sicherheitsanalyse-SynoWake-0.1.10-0011.txt) enthält Belege und Prioritäten. Für diese Analyse habe ich keine Einstellungen oder Paketdateien verändert.
