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

// 只调整独立 SVG 及其单一路径的布局容器，保留作品内部的动画和脚本。
// 完整 HTML/Canvas 仍使用原布局，不能把页面中的装饰 SVG 当作主画布。
const qualityArtworkReceiver = `addEventListener('message',function receive(e){
  if(e.source!==parent||e.data?.type!=='quality-artwork'||typeof e.data.html!=='string'||e.data.html.length>5242880)return;
  removeEventListener('message',receive);
  document.open();document.write(e.data.html);document.close();
  let node=document.body;const containers=[document.documentElement];
  while(node&&node.localName!=='svg'){
    const children=Array.from(node.children).filter(child=>!['script','style','link','meta','title'].includes(child.localName));
    if(children.length!==1||Array.from(node.childNodes).some(child=>child.nodeType===3&&child.textContent.trim()))return;
    containers.push(node);node=children[0];
  }
  if(!node||node.namespaceURI!=='http://www.w3.org/2000/svg')return;
  const box=node.viewBox.baseVal;
  if(!(box.width>0&&box.height>0)){
    const width=node.width.baseVal.value,height=node.height.baseVal.value;
    if(!(width>0&&height>0))return;
    node.setAttribute('viewBox','0 0 '+width+' '+height);
  }
  for(const container of [...containers,node]){
    for(const [key,value] of Object.entries({width:'100%',height:'100%','min-width':'0','min-height':'0','max-width':'none','max-height':'none',margin:'0',padding:'0',border:'0','box-sizing':'border-box',overflow:'hidden',display:'block'}))container.style.setProperty(key,value,'important');
  }
  node.setAttribute('preserveAspectRatio','xMidYMid meet');
});`

func ModelQualityPreview(c *gin.Context) {
	sum := sha256.Sum256([]byte(qualityPreviewBridge))
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'; style-src 'unsafe-inline'; frame-src 'self'; base-uri 'none'; form-action 'none'; sandbox allow-scripts")
	qualityPreviewHeaders(c)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><style>html,body,iframe{margin:0;width:100%;height:100%;border:0;overflow:hidden;background:transparent}</style><iframe title="检测作品" src="/api/v1/model-quality/artwork" sandbox="allow-scripts" referrerpolicy="no-referrer"></iframe><script>`+qualityPreviewBridge+`</script></html>`))
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
