package main

import (
	"archive/zip"
	"fmt"
	"log"

	"embed"
	"encoding/json"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jeandeaual/go-locale"
	zone "github.com/lrstanley/bubblezone"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/tidwall/sjson"
	"golang.org/x/text/language"
)

type model struct {
	voice_chars    []string         // 选项列表
	voice_cursor   int              // 当前高亮的行
	voice_selected map[int]struct{} // 存放已选中的索引
	gui_langs      []string         // 选项列表
	gui_cursor     int              // 当前高亮的行
	gui_selected   map[int]struct{} // 存放已选中的索引
	gui_selected2  int
	paginator      paginator.Model
}

const (
	LIGHT_BULE     lipgloss.Color = lipgloss.Color("#1E90FF")
	GREEN          lipgloss.Color = lipgloss.Color("#008000")
	GRAY           lipgloss.Color = lipgloss.Color("#808080")
	MC_COLOR_PINK  string         = "§d"
	MC_COLOR_RED   string         = "§c"
	MC_COLOR_BOLD  string         = "§l"
	MC_COLOR_GRAY  string         = "§7"
	MC_COLOR_RESET string         = "§r"
)

var Characters = []string{
	"1yoshino", "2mako", "3murasame",
	"4lena", "5koharu", "6roka",
	"7mizuha", "8rentarou", "9genjurou", "$yasuharu"}
var Langs = []string{"zh-CN", "zh-TW", "en-US"}

var FileName2JsonKeyMap = map[string]string{
	"load":    "yuzu_title_button_select_world",
	"system":  "yuzu_title_button_options",
	"goodbye": "yuzu_title_button_quit_game",
	"senren":  "yuzu_title_senren",
	"after":   "yuzu_title_button_realms",
	"extra":   "yuzu_title_button_mod_list",
}
var guiTextures = []string{
	"title_continue_button_normal",
	"title_continue_button_on",
	"title_logo",
	"title_mod_list_button_normal",
	"title_mod_list_button_on",
	"title_new_game_button_normal",
	"title_new_game_button_on",
	"title_options_button_normal",
	"title_options_button_on",
	"title_quit_game_button_normal",
	"title_quit_game_button_on",
	"title_realms_button_normal",
	"title_realms_button_on",
	"title_select_world_button_normal",
	"title_select_world_button_on",
}

var pageItems *[]string
var pageSelected *map[int]struct{}
var pageCursor *int
var pageNo int

//go:embed i18n/*.json
//go:embed sounds/*/*
var embFS embed.FS
var localizer *i18n.Localizer

func InitI18n(locale string) {
	b := i18n.NewBundle(language.SimplifiedChinese)
	b.RegisterUnmarshalFunc("json", json.Unmarshal)

	files, _ := embFS.ReadDir("i18n")

	for _, f := range files {
		data, _ := embFS.ReadFile("i18n/" + f.Name())
		// println(f.Name(), data)
		b.ParseMessageFileBytes(data, f.Name())
	}
	localizer = i18n.NewLocalizer(b, locale)
}

func T(id string) string {
	msg, err := localizer.LocalizeMessage(&i18n.Message{ID: id})
	if err != nil {
		return id // fallback
	}
	return msg
}

func initialModel() model {
	lang, _ := locale.GetLocale()
	InitI18n(lang)
	zone.NewGlobal() // 初始化全局 zone 管理
	pg := paginator.New()
	pg.SetTotalPages(2)
	pg.Type = paginator.Dots
	pg.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"}).Render("•")
	pg.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).Render("•")
	var voice_chars []string
	for _, v := range Characters {
		voice_chars = append(voice_chars, T("char."+v))
	}
	var gui_langs []string
	for _, v := range Langs {
		gui_langs = append(gui_langs, T("lang."+v))
	}

	return model{
		voice_chars:    voice_chars,
		voice_selected: map[int]struct{}{2: {}},
		gui_langs:      gui_langs,
		gui_selected:   map[int]struct{}{0: {}},
		paginator:      pg,
		gui_selected2:  0,
	}
}

func (m model) Init() tea.Cmd {
	pageItems = &m.voice_chars
	pageSelected = &m.voice_selected
	pageCursor = &m.voice_cursor
	pageNo = 0
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.paginator, cmd = m.paginator.Update(msg)
	// print(m.paginator.Page, " ")
	switch msg := msg.(type) {

	// 键盘事件：上下移动、空格切换、q 退出
	case tea.KeyMsg:
		// print("\r", pageNo == m.paginator.Page)
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "ctrl+c":
			fmt.Println("KeyboardInterrupt")
			os.Exit(0)
		case "left", "h", "pageup", "a":
			m.paginator.PrevPage()
			pageNo = 0
			pageItems = &m.voice_chars
			pageSelected = &m.voice_selected
			pageCursor = &m.voice_cursor
		case "right", "l", "pagedown", "d":
			m.paginator.NextPage()
			pageNo = 1
			pageItems = &m.gui_langs
			pageSelected = &m.gui_selected
			pageCursor = &m.gui_cursor
		case "up", "k", "w":
			if *pageCursor > 0 {
				*pageCursor--
			}
		case "down", "j", "s":
			if *pageCursor < len(*pageItems)-1 {
				*pageCursor++
			}
		case " ", "enter":
			if pageNo == 0 {
				if _, ok := (*pageSelected)[*pageCursor]; ok {
					delete(*pageSelected, *pageCursor)
				} else {
					(*pageSelected)[*pageCursor] = struct{}{}
				}
			} else if pageNo == 1 {
				*pageSelected = map[int]struct{}{*pageCursor: {}}
				m.gui_selected2 = *pageCursor
			}
		}

	// 鼠标点击：检测点击是否在某个 zone 内
	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			// Prev 按钮
			if z := zone.Get("btn-prev"); z.InBounds(msg) && pageNo > 0 {
				m.paginator.PrevPage()
				pageNo = 0
				pageItems = &m.voice_chars
				pageSelected = &m.voice_selected
				pageCursor = &m.voice_cursor
				return m, nil
			}
			// Next 按钮
			if z := zone.Get("btn-next"); z.InBounds(msg) && pageNo < m.paginator.TotalPages-1 {
				m.paginator.NextPage()
				pageNo = 1
				pageItems = &m.gui_langs
				pageSelected = &m.gui_selected
				pageCursor = &m.gui_cursor
				return m, nil
			}
			if z := zone.Get("btn-ok"); z.InBounds(msg) {
				return m, tea.Quit
			}
			for i := range *pageItems {
				absIdx := pageNo*100 + i
				z := zone.Get(fmt.Sprint(absIdx))
				// 判断点击是否在该 zone 且在正确的行
				if z.InBounds(msg) && msg.Y == z.StartY {
					if pageNo == 0 {
						if _, ok := (*pageSelected)[i]; ok {
							delete(*pageSelected, i)
						} else {
							(*pageSelected)[i] = struct{}{}
						}
						*pageCursor = i
					} else if pageNo == 1 {
						*pageSelected = map[int]struct{}{i: {}}
						m.gui_selected2 = i
					}
					break
				}
			}
		}
	}

	return m, cmd
}

func (m model) View() string {
	var s string

	selectedStyleNo := lipgloss.NewStyle().
		Foreground(GREEN)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(GRAY)

	if pageNo == 0 {
		s += T("choose.muti.help.head")
	} else if pageNo == 1 {
		s += T("choose.single.help.head")
	}

	s += "\n"

	for i, item := range *pageItems {
		checked := "[ ]"
		text := item
		prefix := " "
		if _, ok := (*pageSelected)[i]; ok {
			checked = selectedStyleNo.Render("[x]")
		} else {
			text = inactiveStyle.Render(item)
		}
		if i == *pageCursor {
			prefix = ">"
		}
		line := fmt.Sprintf("%s %s %s", prefix, checked, text)
		// fmt.Fprintln(file, fmt.Sprint(pageNo*100+i), line)
		s += zone.Mark(fmt.Sprint(pageNo*100+i), line) + "\n"

	}
	s += "\n  " + m.paginator.View() + "\n"
	s += "\n"
	s += zone.Mark("btn-prev", "[ "+T("choose.button.pg.prev")+" ]")
	s += "  "
	s += zone.Mark("btn-next", "[ "+T("choose.button.pg.next")+" ]")
	s += "  "
	s += zone.Mark("btn-ok", "[ "+T("choose.button.pg.ok")+" ]") + "\n"
	s += "\n" + T("choose.help.foot") + "\n"
	return zone.Scan(s)
}

func checkError(e error) {
	if e != nil {
		log.Fatal(e)
		os.Exit(1)
	}
}

var zipWriter *zip.Writer

func main() {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // 开启鼠标事件
	)
	mo, err := p.Run()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	m := mo.(model)
	var ZipFileName string
	var isReplace bool
	TargetLocale := Langs[m.gui_selected2]
	if _, exists := m.voice_selected[2]; exists {
		ZipFileName = MC_COLOR_PINK + "OUTPUT" + MC_COLOR_GRAY + "-" + MC_COLOR_GRAY + "-" + MC_COLOR_RESET + "YuZuUI" + ".zip"
		isReplace = false
	} else {
		ZipFileName = MC_COLOR_PINK + "OUTPUT" + MC_COLOR_GRAY + "-" + MC_COLOR_RED + MC_COLOR_BOLD + "REPLACE" + MC_COLOR_GRAY + "-" + MC_COLOR_RESET + "YuZuUI" + ".zip"
		isReplace = true
	}
	if _, err := os.Stat(ZipFileName); err == nil {
		os.Remove(ZipFileName)
	}
	newZipFile, err := os.Create(ZipFileName)
	checkError(err)
	defer newZipFile.Close()
	zipWriter = zip.NewWriter(newZipFile)
	defer zipWriter.Close()
	sounds_json_b, _ := embFS.ReadFile("sounds/others/sounds.json")
	sounds_json := string(sounds_json_b)
	if isReplace {
		sounds_json, _ = sjson.Set(sounds_json, "yuzu_title_button_select_world.replace", true)
		sounds_json, _ = sjson.Set(sounds_json, "yuzu_title_button_options.replace", true)
		sounds_json, _ = sjson.Set(sounds_json, "yuzu_title_button_quit_game.replace", true)
		sounds_json, _ = sjson.Set(sounds_json, "yuzu_title_senren.replace", true)
		sounds_json, _ = sjson.Set(sounds_json, "yuzu_title_button_realms.replace", true)
		sounds_json, _ = sjson.Set(sounds_json, "yuzu_title_button_mod_list.replace", true)
	}
	for k := range m.voice_selected {
		if k == 2 {
			continue
		}
		charWithNo := Characters[k]
		char := charWithNo[1:]

		for FileName, JsonKey := range FileName2JsonKeyMap {
			key := fmt.Sprintf("%s/%s_%s", charWithNo, char, FileName)
			ReadAndWrite("sounds/"+key+".ogg", "assets/yuzu/sounds/"+key+".ogg")
			JsonAppend(&sounds_json, JsonKey+".sounds", "yuzu:"+key)
		}
	}
	w, _ := zipWriter.Create("assets/yuzu/sounds.json")
	_, _ = io.WriteString(w, sounds_json)
	for k := range guiTextures {
		switch TargetLocale {
		case "zh-TW":
			ReadAndWrite(
				"sounds/others/textures/gui/"+TargetLocale+"/"+guiTextures[k]+".png",
				"assets/yuzu/textures/gui/"+guiTextures[k]+".png")
		case "en-US":
			ReadAndWrite(
				"sounds/others/textures/gui/"+TargetLocale+"/"+guiTextures[k]+".png",
				"assets/yuzu/textures/gui/"+guiTextures[k]+".png")
		}
	}
	ReadAndWrite("sounds/others/pack.mcmeta", "pack.mcmeta")
	ReadAndWrite("sounds/others/pack.png", "pack.png")
}
func JsonAppend(json *string, path string, value string) {
	*json, _ = sjson.Set(*json, path+".-1", value)
}
func ReadAndWrite(path1 string, path2 string) {
	f, _ := embFS.Open(path1)
	defer f.Close()
	w, _ := zipWriter.Create(path2)
	_, _ = io.Copy(w, f)
}
