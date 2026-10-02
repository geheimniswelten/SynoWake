# Abnahme auf DS918+ / DSM 7.1

Diese Liste ist eine Anleitung für die Zielsystemprüfung. Ein lokaler Build oder eine Browser-Demo bestätigt keine DSM-Installation. Vor dem Test die tatsächliche DSM-Version und die NAS-Zeitzone notieren. Ein einziger Testrechner mit bekannter MAC-Adresse und funktionierendem Wake-on-LAN genügt für den ersten Durchlauf.

| Prüfung | Erwartetes Ergebnis |
| --- | --- |
| SPK manuell installieren | Paket-Zentrum akzeptiert die Architektur und Mindestversion; keine Root-Privilege-Meldung. |
| Installierte Programmrechte prüfen | `ui/api.cgi` und `bin/synowake` haben Modus `0755`; keine Setuid-/Setgid-Bits und keine zusätzlichen Ausführungsprivilegien. |
| Paketdienst prüfen | Dienst läuft unter dem SynoWake-Paketkonto; `target/run/backend.sock` existiert und `bin/synowake status` liefert 0. |
| Paket starten und öffnen | Eigenes DSM-Fenster zeigt Gerätekacheln, Geräteliste und Automatik. |
| Zugriff ohne DSM-Anmeldung | `api.cgi` verweigert Daten und Wake-Aktionen. |
| Zugriff als normaler DSM-Benutzer | Backend verweigert die Administrationsfunktionen. |
| Gerät mit Name, IP und MAC speichern | Zeile und Kachel erscheinen; Name lässt sich ändern. |
| Falsche IP oder MAC eingeben | Sichtbare Validierungsfehlermeldung; kein Wake-Paket. |
| Lokales `/24` suchen | Erreichbare LAN-Nachbarn mit MAC erscheinen; Treffer lassen sich übernehmen. |
| Einzelgerät und Schnellauswahl starten | Magic Packet erreicht den Testrechner; Status wechselt von „Wird aufgeweckt“ nach „Online“, wenn ICMP beantwortet wird. |
| Onlineprüfung mit gesperrtem ICMP | Gerät wird nicht als nachweislich online angezeigt; Einschränkung ist nachvollziehbar. |
| Zeitplan für die nächste Minute speichern | Im DSM-Aufgabenplaner existiert genau eine zugehörige SynoWake-Aufgabe. |
| Zeitplan ausführen | Testrechner startet; Anwendungsprotokoll enthält Zeit und Gerät. |
| Uhrzeit und Wochentage bearbeiten | Bestehende DSM-Aufgabe wird geändert; keine doppelte Aufgabe. |
| Zeitplan deaktivieren/aktivieren | DSM-Aufgabe und Oberflächenzustand stimmen überein. |
| Benachrichtigung aktivieren | DSM-Desktopmeldung erscheint bei der Ausführung; bei ausgeschaltetem Haken fehlt sie. |
| Protokoll-Center öffnen | Die Wake-Aktion ist dort mit SynoWake-Kennung nachvollziehbar. Eine Diagnose in SynoWake darf nicht übergangen werden. |
| TCP-Empfänger als Ersatzweg einstellen | BSD/TCP-Empfang im Protokoll-Center und gespeicherter SynoWake-Port stimmen überein; Loopback-Versand erscheint im Protokoll-Center. |
| Empfang vorübergehend ausschalten | Neue Wake-Aktion bleibt im lokalen Verlauf als nicht übermittelt markiert; eine Diagnose ist sichtbar. |
| Empfang wieder einschalten und erneut senden | Vorgemerkter Eintrag wird übertragen; keine Wake-Aktion wird dadurch erneut ausgelöst. |
| Rückstau im Ausführungsprotokoll prüfen | Ausstehende Einträge werden beim Begrenzen älterer übertragener Einträge erhalten; bei 3000 ausstehenden Einträgen werden neue Wake-Aktionen vor dem Versand angehalten. |
| Paket stoppen | Ausführung eines vorhandenen Zeitplans sendet kein Magic Packet. |
| Paket wieder starten | Der Zeitplan kann wieder ausführen. |
| NAS neu starten | Paket und registrierte Aufgaben arbeiten nach dem Start weiter. |
| Paket aktualisieren | Geräte, Zeitpläne und Protokoll bleiben erhalten; Aufgaben werden nicht dupliziert. |
| Alle SynoWake-Zeitpläne entfernen | Zugehörige Aufgaben verschwinden auch aus dem DSM-Aufgabenplaner. |
| Paket anschließend deinstallieren | Kein SynoWake-Programmpfad und keine zugehörige Aufgabe bleiben zur Ausführung registriert. |

## Diagnose bei einem Fehler

Das sichtbare Fehlerbild, die DSM-Version, die Uhrzeit und die Aktion festhalten. Im SynoWake-Protokoll die Diagnose ansehen. Bei Problemen mit Installation oder Start zusätzlich `/var/log/packages/SynoWake.log` und die DSM-Paketbetriebsprotokolle prüfen. Die private Dateiablage in `/var/packages/SynoWake/var` enthält Zeitplan-Geheimnisse und gehört nicht in ungeschwärzte Supportberichte.

Bei einer Aufgabenplaner-WebAPI-Meldung den Fehlercode und die Methode erfassen. Den betreffenden Zeitplan auch direkt im DSM-Aufgabenplaner kontrollieren. Eine fehlgeschlagene Registrierung bleibt ein Fehler; den Zeitplan nicht als funktionierend abnehmen.

Wenn das native Fenster leer bleibt, zunächst die direkte UI-Adresse nach DSM-Anmeldung öffnen. Wenn die direkte Oberfläche funktioniert, liegt der Fehler bei der DSM-Fensterregistrierung. Wenn auch `api.cgi` fehlschlägt, Dienststatus, `/var/packages/SynoWake/var/service.log`, CGI-Ausführbarkeit und Authentifizierungsprüfung untersuchen. Die CGI-Weiterleitung muss den Socket erreichen, darf aber keine privaten Datendateien benötigen.

Wenn die Wake-Aktion im eigenen Verlauf erscheint, aber nicht im Protokoll-Center, die Diagnose für `synologset1` prüfen. Im Protokoll-Center zunächst ein Archivziel wählen und einen **BSD/TCP**-Empfänger erstellen. Dessen Port in SynoWake einstellen. Für den Loopback-Sender kein SSL aktivieren. Eine neue Aktion ausführen und die Log-Center-Anzeige prüfen. Nach Beheben des Empfängers vorgemerkte Einträge erneut senden. Ein erfolgreicher TCP-Versand bestätigt noch keinen Datenbankeintrag; diese Sichtprüfung ist Teil der Abnahme.

## Testergebnis erfassen

```text
NAS-Modell: DS918+
DSM-Version/Build:
NAS-Zeitzone:
SynoWake-Version:
SPK-SHA256:
Installation / CGI / Fenster:
Geräte / Suche / Wake / Status:
Zeitplan anlegen / editieren / deaktivieren / löschen:
Stop / Start / Neustart / Upgrade:
Protokoll-Center:
DSM-Benachrichtigungen:
Verbliebene Aufgaben nach Entfernen:
Offene Fehler:
```
