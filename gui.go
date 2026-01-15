//go:build !cli

/*
 * SPDX-License-Identifier: GPL-3.0
 * Mushcord Installer, a cross platform gui/cli app for installing Mushcord
 * Copyright (c) 2023 Vendicated and Vencord contributors
 */

package main

import (
	"bytes"
	_ "embed"
	"errors"
	"image"
	"image/color"
	"vencord/buildinfo"

	g "github.com/AllenDang/giu"
	"github.com/AllenDang/imgui-go"

	// png decoder for icon
	_ "image/png"
	"os"
	path "path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	discords        []any
	radioIdx        int
	customChoiceIdx int

	customDir              string
	autoCompleteDir        string
	autoCompleteFile       string
	autoCompleteCandidates []string
	autoCompleteIdx        int
	lastAutoComplete       string
	didAutoComplete        bool

	modalId      = 0
	modalTitle   = "Oh Non :("
	modalMessage = "Vous ne devriez jamais voir ce message"

	acceptedOpenAsar   bool
	showedUpdatePrompt bool

	win *g.MasterWindow
)

//go:embed winres/icon.png
var iconBytes []byte

func init() {
	LogLevel = LevelDebug
}

func main() {
	InitGithubDownloader()
	discords = FindDiscords()

	customChoiceIdx = len(discords)

	go func() {
		<-GithubDoneChan
		g.Update()
	}()

	go func() {
		<-SelfUpdateCheckDoneChan
		g.Update()
	}()

	var linuxFlags g.MasterWindowFlags = 0
	if runtime.GOOS == "linux" {
		os.Setenv("GDK_SCALE", "1")
		os.Setenv("GDK_DPI_SCALE", "1")
	}

	win = g.NewMasterWindow("Mushcord Installer", 1200, 800, linuxFlags)

	icon, _, err := image.Decode(bytes.NewReader(iconBytes))
	if err != nil {
		Log.Warn("Failed to load application icon", err)
		Log.Debug(iconBytes, len(iconBytes))
	} else {
		win.SetIcon([]image.Image{icon})
	}
	win.Run(loop)
}

type CondWidget struct {
	predicate  bool
	ifWidget   func() g.Widget
	elseWidget func() g.Widget
}

func (w *CondWidget) Build() {
	if w.predicate {
		w.ifWidget().Build()
	} else if w.elseWidget != nil {
		w.elseWidget().Build()
	}
}

func getChosenInstall() *DiscordInstall {
	var choice *DiscordInstall
	if radioIdx == customChoiceIdx {
		choice = ParseDiscord(customDir, "")
		if choice == nil {
			g.OpenPopup("#invalid-custom-location")
		}
	} else {
		choice = discords[radioIdx].(*DiscordInstall)
	}
	return choice
}

func InstallLatestBuilds() (err error) {
	if IsDevInstall {
		return
	}

	err = installLatestBuilds()
	if err != nil {
		ShowModal("Uh Oh!", "Échec de l'installation des derniers builds de Mushcord depuis GitHub :\n"+err.Error())
	}
	return
}

func handlePatch() {
	choice := getChosenInstall()
	if choice != nil {
		choice.Patch()
	}
}

func handleUnpatch() {
	choice := getChosenInstall()
	if choice != nil {
		choice.Unpatch()
	}
}

func handleOpenAsar() {
	if acceptedOpenAsar || getChosenInstall().IsOpenAsar() {
		handleOpenAsarConfirmed()
		return
	}

	g.OpenPopup("#openasar-confirm")
}

func handleOpenAsarConfirmed() {
	choice := getChosenInstall()
	if choice != nil {
		if choice.IsOpenAsar() {
			if err := choice.UninstallOpenAsar(); err != nil {
				handleErr(choice, err, "désinstaller OpenAsar de")
			} else {
				g.OpenPopup("#openasar-unpatched")
				g.Update()
			}
		} else {
			if err := choice.InstallOpenAsar(); err != nil {
				handleErr(choice, err, "installer OpenAsar sur")
			} else {
				g.OpenPopup("#openasar-patched")
				g.Update()
			}
		}
	}
}

func handleErr(di *DiscordInstall, err error, action string) {
	if errors.Is(err, os.ErrPermission) {
		switch runtime.GOOS {
		case "windows":
			err = errors.New("Permission refusée. Assurez-vous que Discord est totalement fermé (même dans la barre des tâches) !")
		case "darwin":
			command := "sudo chown -R \"${USER}:wheel\" " + di.path
			err = errors.New("Permission refusée. Veuillez accorder l'accès complet au disque à l'installateur dans les réglages système.\n\nSi cela ne fonctionne pas, essayez cette commande :\n" + command)
		case "linux":
			command := "sudo chown -R \"$USER:$USER\" " + di.path
			err = errors.New("Permission refusée. Essayez de lancer l'installateur avec les privilèges sudo.\n\nCommande recommandée :\n" + command)
		default:
			err = errors.New("Permission refusée. Essayez de lancer en tant qu'Administrateur.")
		}
	}

	ShowModal("Échec de l'action : "+action, err.Error())
}

func HandleScuffedInstall() {
	g.OpenPopup("#scuffed-install")
}

func (di *DiscordInstall) Patch() {
	if CheckScuffedInstall() {
		return
	}
	if err := di.patch(); err != nil {
		handleErr(di, err, "patch")
	} else {
		g.OpenPopup("#patched")
	}
}

func (di *DiscordInstall) Unpatch() {
	if err := di.unpatch(); err != nil {
		handleErr(di, err, "unpatch")
	} else {
		g.OpenPopup("#unpatched")
	}
}

func onCustomInputChanged() {
	p := customDir
	if len(p) != 0 {
		radioIdx = customChoiceIdx
	}

	dir := path.Dir(p)
	isNewDir := strings.HasSuffix(p, "/")
	wentUpADir := !isNewDir && dir != autoCompleteDir

	if isNewDir || wentUpADir {
		autoCompleteDir = dir
		autoCompleteIdx = 0
		lastAutoComplete = ""
		autoCompleteFile = ""
		autoCompleteCandidates = nil

		files, err := os.ReadDir(dir)
		if err == nil {
			for _, file := range files {
				autoCompleteCandidates = append(autoCompleteCandidates, file.Name())
			}
		}
	} else if !didAutoComplete {
		autoCompleteFile = path.Base(p)
		lastAutoComplete = ""
	}

	if wentUpADir {
		autoCompleteFile = path.Base(p)
	}

	didAutoComplete = false
}

func makeAutoComplete() []any {
	input := strings.ToLower(autoCompleteFile)
	var candidates []any
	for _, e := range autoCompleteCandidates {
		file := strings.ToLower(e)
		if autoCompleteFile == "" || strings.HasPrefix(file, input) {
			candidates = append(candidates, e)
		}
	}
	return candidates
}

func makeRadioOnChange(i int) func() {
	return func() {
		radioIdx = i
	}
}

func Tooltip(label string) g.Widget {
	return g.Style().
		SetStyle(g.StyleVarWindowPadding, 10, 8).
		SetStyleFloat(g.StyleVarWindowRounding, 8).
		To(
			g.Tooltip(label),
		)
}

func InfoModal(id, title, description string) g.Widget {
	return RawInfoModal(id, title, description, false)
}

func RawInfoModal(id, title, description string, isOpenAsar bool) g.Widget {
	isDynamic := strings.HasPrefix(id, "#modal") && !strings.Contains(description, "\n")
	return g.Style().
		SetStyle(g.StyleVarWindowPadding, 30, 30).
		SetStyleFloat(g.StyleVarWindowRounding, 12).
		To(
			g.PopupModal(id).
				Flags(g.WindowFlagsNoTitleBar | Ternary(isDynamic, g.WindowFlagsAlwaysAutoResize, 0)).
				Layout(
					g.Align(g.AlignCenter).To(
						g.Style().SetFontSize(30).To(
							g.Label(title),
						),
						g.Style().SetFontSize(20).To(
							g.Label(description).Wrapped(isDynamic),
						),
						&CondWidget{id == "#scuffed-install", func() g.Widget {
							return g.Column(
								g.Dummy(0, 10),
								g.Button("Emmenez-moi là-bas !").OnClick(func() {
									username := os.Getenv("USERNAME")
									programData := os.Getenv("PROGRAMDATA")
									g.OpenURL("file://" + path.Join(programData, username))
								}).Size(200, 30),
							)
						}, nil},
						g.Dummy(0, 20),
						&CondWidget{isOpenAsar,
							func() g.Widget {
								return g.Row(
									g.Button("Accepter").
										OnClick(func() {
											acceptedOpenAsar = true
											g.CloseCurrentPopup()
										}).
										Size(100, 30),
									g.Button("Annuler").
										OnClick(func() {
											g.CloseCurrentPopup()
										}).
										Size(100, 30),
								)
							},
							func() g.Widget {
								return g.Button("Ok").
									OnClick(func() {
										g.CloseCurrentPopup()
									}).
									Size(100, 30)
							},
						},
					),
				),
		)
}

func UpdateModal() g.Widget {
	return g.Style().
		SetStyle(g.StyleVarWindowPadding, 30, 30).
		SetStyleFloat(g.StyleVarWindowRounding, 12).
		To(
			g.PopupModal("#update-prompt").
				Flags(g.WindowFlagsNoTitleBar | g.WindowFlagsAlwaysAutoResize).
				Layout(
					g.Align(g.AlignCenter).To(
						g.Style().SetFontSize(30).To(
							g.Label("Votre installateur est obsolète !"),
						),
						g.Style().SetFontSize(20).To(
							g.Label(
								"Voulez-vous mettre à jour maintenant ?\n\n"+
									"Le nouvel installateur sera téléchargé automatiquement.\n"+
									"L'outil peut sembler figé quelques instants, patientez !\n"+
									"Une fois terminé, Mushcord Installer redémarrera seul.",
							),
						),
						g.Row(
							g.Button("Mettre à jour").
								OnClick(func() {
									if runtime.GOOS == "darwin" {
										g.CloseCurrentPopup()
										g.OpenURL(GetInstallerDownloadLink())
										return
									}

									err := UpdateSelf()
									g.CloseCurrentPopup()

									if err != nil {
										ShowModal("Échec de la mise à jour !", err.Error())
									} else {
										if err = RelaunchSelf(); err != nil {
											ShowModal("Échec du redémarrage ! Faites-le manuellement.", err.Error())
										}
									}
								}).
								Size(120, 30),
							g.Button("Plus tard").
								OnClick(func() {
									g.CloseCurrentPopup()
								}).
								Size(100, 30),
						),
					),
				),
		)
}

func ShowModal(title, desc string) {
	modalTitle = title
	modalMessage = desc
	modalId++
	g.OpenPopup("#modal" + strconv.Itoa(modalId))
}

func renderInstaller() g.Widget {
	candidates := makeAutoComplete()
	wi, _ := win.GetSize()
	w := float32(wi) - 96

	var currentDiscord *DiscordInstall
	if radioIdx != customChoiceIdx {
		currentDiscord = discords[radioIdx].(*DiscordInstall)
	}
	var isOpenAsar = currentDiscord != nil && currentDiscord.IsOpenAsar()

	if CanUpdateSelf() && !showedUpdatePrompt {
		showedUpdatePrompt = true
		g.OpenPopup("#update-prompt")
	}

	layout := g.Layout{
		g.Dummy(0, 20),
		g.Separator(),
		g.Dummy(0, 5),

		g.Style().SetFontSize(20).To(
			renderErrorCard(
				DiscordYellow,
				"**Mushcord** et son dépôt GitHub officiel sont les seuls endroits sûrs pour obtenir ce mod. Toute autre source est potentiellement malveillante. **Ceci est l'installeur officiel de Mushcord, créé par MushZi (dsc: MushZi)**.\n"+
					"Si vous avez téléchargé cet outil ailleurs, par précaution, supprimez tout, effectuez un scan antivirus et changez votre mot de passe Discord.",
				110,
			),
		),

		g.Dummy(0, 5),

		g.Style().SetFontSize(30).To(
			g.Label("Veuillez sélectionner une version de Discord"),
		),

		&CondWidget{len(discords) == 0, func() g.Widget {
			s := "Aucune installation de Discord trouvée. Installez Discord d'abord."
			if runtime.GOOS == "linux" {
				s += " Les versions Snap ne sont pas supportées."
			}
			return g.Label(s)
		}, nil},

		g.Style().SetFontSize(20).To(
			g.RangeBuilder("Discords", discords, func(i int, v any) g.Widget {
				d := v.(*DiscordInstall)
				text := strings.Title(d.branch) + " - " + d.path
				if d.isPatched {
					text += " [PATCHÉ]"
				}
				return g.RadioButton(text, radioIdx == i).
					OnChange(makeRadioOnChange(i))
			}),

			g.RadioButton("Emplacement personnalisé", radioIdx == customChoiceIdx).
				OnChange(makeRadioOnChange(customChoiceIdx)),
		),

		g.Dummy(0, 5),
		g.Style().
			SetStyle(g.StyleVarFramePadding, 16, 16).
			SetFontSize(20).
			To(
				g.InputText(&customDir).Hint("Chemin personnalisé").
					Size(w - 16).
					Flags(g.InputTextFlagsCallbackCompletion).
					OnChange(onCustomInputChanged).
					Callback(
						func(data imgui.InputTextCallbackData) int32 {
							if len(candidates) == 0 {
								return 0
							}
							if autoCompleteIdx >= len(candidates) {
								autoCompleteIdx = 0
							}
							didAutoComplete = true
							start := len(customDir)
							if lastAutoComplete != "" {
								start -= len(lastAutoComplete)
								data.DeleteBytes(start, len(lastAutoComplete))
							} else if autoCompleteFile != "" {
								start -= len(autoCompleteFile)
								data.DeleteBytes(start, len(autoCompleteFile))
							}
							lastAutoComplete = candidates[autoCompleteIdx].(string)
							data.InsertBytes(start, []byte(lastAutoComplete))
							autoCompleteIdx++
							return 0
						},
					),
			),
		g.RangeBuilder("AutoComplete", candidates, func(i int, v any) g.Widget {
			dir := v.(string)
			return g.Label(dir)
		}),

		g.Dummy(0, 20),

		g.Style().SetFontSize(20).To(
			g.Row(
				g.Style().
					SetColor(g.StyleColorButton, DiscordGreen).
					SetDisabled(GithubError != nil).
					To(
						g.Button("Installer").
							OnClick(handlePatch).
							Size((w-40)/4, 50),
						Tooltip("Appliquer Mushcord sur la version sélectionnée"),
					),
				g.Style().
					SetColor(g.StyleColorButton, DiscordBlue).
					SetDisabled(GithubError != nil).
					To(
						g.Button("Réparer").
							OnClick(func() {
								if IsDevInstall {
									handlePatch()
								} else {
									err := InstallLatestBuilds()
									if err == nil {
										handlePatch()
									}
								}
							}).
							Size((w-40)/4, 50),
						Tooltip("Réinstaller et mettre à jour Mushcord"),
					),
				g.Style().
					SetColor(g.StyleColorButton, DiscordRed).
					To(
						g.Button("Désinstaller").
							OnClick(handleUnpatch).
							Size((w-40)/4, 50),
						Tooltip("Retirer Mushcord de Discord"),
					),
				g.Style().
					SetColor(g.StyleColorButton, Ternary(isOpenAsar, DiscordRed, DiscordGreen)).
					To(
						g.Button(Ternary(isOpenAsar, "Désinstaller OpenAsar", "Installer OpenAsar")).
							OnClick(handleOpenAsar).
							Size((w-40)/4, 50),
						Tooltip("Gérer OpenAsar"),
					),
			),
		),

		InfoModal("#patched", "Patch Réussi", "Si Discord est ouvert, fermez-le complètement.\n"+
			"Lancez-le ensuite et vérifiez que Mushcord est présent dans les paramètres."),
		InfoModal("#unpatched", "Désinstallation Réussie", "Discord a été remis à neuf !"),
		InfoModal("#scuffed-install", "Attention !", "Votre installation de Discord semble corrompue ou mal placée.\n"+
			"Veuillez supprimer les dossiers 'Discord' ou 'Squirrel' dans le dossier qui va s'ouvrir, puis réinstallez Discord."),
		RawInfoModal("#openasar-confirm", "OpenAsar", "OpenAsar est une alternative open-source.\n"+
			"Mushcord n'est pas affilié à OpenAsar. Installation à vos risques et périls.\n\n"+
			"Pour continuer, cliquez sur Accepter puis sur 'Installer OpenAsar' à nouveau.", true),
		InfoModal("#openasar-patched", "OpenAsar Installé", "Vérifiez l'installation après avoir relancé Discord !"),
		InfoModal("#openasar-unpatched", "OpenAsar Retiré", "Discord est revenu à sa version d'origine."),
		InfoModal("#invalid-custom-location", "Emplacement Invalide", "Ce dossier ne contient pas une version valide de Discord."),
		InfoModal("#modal"+strconv.Itoa(modalId), modalTitle, modalMessage),

		UpdateModal(),
	}

	return layout
}

func renderErrorCard(col color.Color, message string, height float32) g.Widget {
	return g.Style().
		SetColor(g.StyleColorChildBg, col).
		SetStyleFloat(g.StyleVarAlpha, 0.9).
		SetStyle(g.StyleVarWindowPadding, 10, 10).
		SetStyleFloat(g.StyleVarChildRounding, 5).
		To(
			g.Child().
				Size(g.Auto, height).
				Layout(
					g.Row(
						g.Style().SetColor(g.StyleColorText, color.Black).To(
							g.Markdown(&message),
						),
					),
				),
		)
}

func loop() {
	g.PushWindowPadding(48, 48)

	g.SingleWindow().
		RegisterKeyboardShortcuts(
			g.WindowShortcut{Key: g.KeyUp, Callback: func() {
				if radioIdx > 0 {
					radioIdx--
				}
			}},
			g.WindowShortcut{Key: g.KeyDown, Callback: func() {
				if radioIdx < customChoiceIdx {
					radioIdx++
				}
			}},
		).
		Layout(
			g.Align(g.AlignCenter).To(
				g.Style().SetFontSize(40).To(
					g.Label("Mushcord Installer"),
				),
			),

			g.Dummy(0, 20),
			g.Style().SetFontSize(20).To(
				g.Row(
					g.Label("Mushcord sera téléchargé dans : "+EquicordDirectory),
					g.Style().
						SetColor(g.StyleColorButton, DiscordBlue).
						SetStyle(g.StyleVarFramePadding, 4, 4).
						To(
							g.Button("Ouvrir le dossier").OnClick(func() {
								g.OpenURL("file://" + path.Dir(EquicordDirectory))
							}),
						),
				),
				&CondWidget{!IsDevInstall, func() g.Widget {
					return g.Label("Pour changer cela, définissez la variable d'environnement 'EQUICORD_USER_DATA_DIR'.").Wrapped(true)
				}, nil},
				g.Dummy(0, 10),
				g.Label("Version de l'installateur : "+buildinfo.InstallerTag+" ("+buildinfo.InstallerGitHash+")"),
				g.Label("Version locale de Mushcord : "+InstalledHash),
				&CondWidget{
					GithubError == nil,
					func() g.Widget {
						return g.Label("Dernière version disponible : " + LatestHash)
					}, func() g.Widget {
						return renderErrorCard(DiscordRed, "Erreur GitHub : "+GithubError.Error(), 40)
					},
				},
			),

			renderInstaller(),
		)

	g.PopStyle()
}
