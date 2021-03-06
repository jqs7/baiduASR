package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/emicklei/go-restful"
	"github.com/google/uuid"
	"github.com/levigross/grequests"
	"github.com/tuotoo/biu"
	"github.com/tuotoo/biu/box"
	"github.com/tuotoo/biu/log"
	"github.com/tuotoo/biu/opt"
)

type BaiduAI struct{}

type BaiduAudio struct {
	Format  string `json:"format"`
	Rate    int    `json:"rate"`
	DevPid  int    `json:"dev_pid"`
	Channel int    `json:"channel"`
	Token   string `json:"token"`
	Cuid    string `json:"cuid"`
	Len     int    `json:"len"`
	Speech  string `json:"speech"`
}

func (ctl BaiduAI) WebService(ws biu.WS) {
	ws.Route(ws.POST("/audio").
		Doc("百度语音识别").
		Consumes(biu.MIME_FILE_FORM).
		Param(ws.FormParameter("file", "文件").DataType("file").Required(true)).
		Param(ws.FormParameter("token", "百度接口 token").Required(true)).
		Param(ws.FormParameter("devPid", "识别模式").DataType("integer")).
		Param(ws.FormParameter("cuid", "用户标识")),
		opt.RouteID("baidu.ai.audio"),
		opt.RouteTo(ctl.audio),
		opt.RouteErrors(map[int]string{
			100: "获取文件失败",
			101: "打开文件失败",
			102: "创建文件失败",
			103: "复制文件内容失败",
			104: "文件转码失败",
			105: "读取文件数据失败",
			106: "获取 token 失败",
			107: "访问百度 AI 接口失败",
		}),
	)
}

func (ctl BaiduAI) audio(ctx box.Ctx) {
	_, fh, err := ctx.Request.Request.FormFile("file")
	ctx.Must(err, 100)

	f, err := fh.Open()
	ctx.Must(err, 101)
	defer f.Close()

	filename := uuid.New().String()

	nf, err := os.Create(filename)
	ctx.Must(err, 102)
	defer nf.Close()

	_, err = io.Copy(nf, f)
	ctx.Must(err, 103)

	pcmFilename := filename + ".pcm"
	_, err = exec.Command("ffmpeg", "-y",
		"-i", filename,
		"-acodec", "pcm_s16le",
		"-f", "s16le",
		"-ac", "1",
		"-ar", "16000",
		pcmFilename).Output()
	ctx.Must(err, 104)

	token, err := ctx.Form("token").String()
	ctx.Must(err, 106)

	pcmData, err := ioutil.ReadFile(pcmFilename)
	ctx.Must(err, 105)

	cuid, _ := ctx.Form("cuid").String()
	if cuid == "" {
		cuid = "nyan"
	}

	devPID, _ := ctx.Form("devPid").Int()
	if devPID != 1536 && devPID != 1537 && devPID != 1737 &&
		devPID != 1637 && devPID != 1837 && devPID != 1936 {
		devPID = 1537
	}

	pcmBase64 := base64.StdEncoding.EncodeToString(pcmData)
	resp, err := grequests.Post("http://vop.baidu.com/server_api",
		&grequests.RequestOptions{
			JSON: BaiduAudio{
				Format:  "pcm",
				Rate:    16000,
				DevPid:  devPID,
				Channel: 1,
				Token:   token,
				Cuid:    cuid,
				Len:     len(pcmData),
				Speech:  pcmBase64,
			},
		})
	ctx.Must(err, 107)

	os.Remove(filename)
	os.Remove(pcmFilename)

	ctx.ResponseJSON(json.RawMessage(resp.Bytes()))
}

func main() {
	log.UseColorLogger()
	restful.Filter(biu.LogFilter())
	biu.AddServices("/v1", nil,
		biu.NS{
			NameSpace:  "baidu",
			Controller: BaiduAI{},
			Desc:       "百度 AI",
		},
	)
	swaggerService := biu.NewSwaggerService(biu.SwaggerInfo{
		Version:     "1.0.0",
		RoutePrefix: "/v1",
	})
	restful.Add(swaggerService)
	biu.Run(":7093")
}
