package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 两层空壳均不读取认证信息。外层只转发父窗口已鉴权取得的作品，
// frame-src 额外限制内层的自身导航，弥补 connect-src 无法约束导航的缺口。
const qualityPreviewBridge = `const frame=document.querySelector('iframe');let delivered=false;frame.onload=()=>{if(!delivered)parent.postMessage({type:'quality-ready'},'*')};addEventListener('message',e=>{if(e.source!==parent||delivered||e.data?.type!=='quality-artwork'||typeof e.data.html!=='string'||e.data.html.length>5242880)return;delivered=true;frame.contentWindow.postMessage({type:'quality-artwork',html:e.data.html},'*')});`
const qualityArtworkReceiver = `addEventListener('message',function receive(e){if(e.source!==parent||e.data?.type!=='quality-artwork'||typeof e.data.html!=='string'||e.data.html.length>5242880)return;removeEventListener('message',receive);document.open();document.write(e.data.html);document.close()});`

func ModelQualityPreview(c *gin.Context) {
	sum := sha256.Sum256([]byte(qualityPreviewBridge))
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'; style-src 'unsafe-inline'; frame-src 'self'; base-uri 'none'; form-action 'none'; sandbox allow-scripts")
	qualityPreviewHeaders(c)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><style>html,body,iframe{margin:0;width:100%;height:100%;border:0;overflow:hidden;background:transparent}</style><iframe title="鹈鹕作品" src="/api/v1/model-quality/artwork" sandbox="allow-scripts" referrerpolicy="no-referrer"></iframe><script>`+qualityPreviewBridge+`</script></html>`))
}
func ModelQualityArtwork(c *gin.Context) {
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; connect-src 'none'; frame-src 'none'; worker-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; sandbox allow-scripts")
	qualityPreviewHeaders(c)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(fmt.Sprintf(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><script>%s</script></html>`, qualityArtworkReceiver)))
}
func qualityPreviewHeaders(c *gin.Context) {
	c.Writer.Header().Del("X-Frame-Options")
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), clipboard-read=(), clipboard-write=()")
}
