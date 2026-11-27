"use strict";
const $ = (selector) => document.querySelector(selector);
const labels = {queued:"排队中",running:"检测中",cancelling:"停止中",cancelled:"已停止",finished:"已完成",failed:"执行失败",interrupted:"已中断",completed:"检查完成",observations:"存在差异",incomplete:"部分无法确认",pass:"通过",fail:"不通过",unknown:"无法确认",skipped:"已跳过",accepted:"完整接受",rejected_http_400:"HTTP400拒绝",rejected_stream_error:"流内拒绝",inconclusive:"无法判定",source_valid:"有效来源",source_invalid:"来源无效",rejected_signature:"HTTP400拒签"};
const testNames = {conversation:"三轮缓存",fingerprint:"Token指纹",signature_freshness:"实时签名",signature_relay:"工具续接",signature:"伪造签名",concurrency:"并发能力"};
const relayNames = {seed:"取得来源",original_before:"前置对照",original_after:"后置对照",changed_question:"修改原题",cross:"跨渠道"};
let selected = null, current = null, authenticated = false, polling = false, historyJobs = [];
const esc = (v) => String(v ?? "—").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
const num = (v, digits=0) => typeof v === "number" ? v.toLocaleString("zh-CN",{maximumFractionDigits:digits,minimumFractionDigits:digits}) : "—";
const date = (v) => v ? new Date(typeof v === "number" ? v * 1000 : v).toLocaleString("zh-CN",{timeZone:"Asia/Shanghai",hour12:false}) : "—";
const tag = (value, text) => `<span class="tag ${esc(value)}">${esc(text || labels[value] || value)}</span>`;
const icons = () => window.lucide?.createIcons();
function table(head, rows) {return `<div class="table-scroll"><table><thead><tr>${head.map(h=>`<th>${esc(h)}</th>`).join("")}</tr></thead><tbody>${rows.length ? rows.map(r=>`<tr>${r.map(c=>`<td>${c}</td>`).join("")}</tr>`).join("") : `<tr><td class="muted-cell" colspan="${head.length}">暂无记录</td></tr>`}</tbody></table></div>`;}
function error(message) {$("#global-error").textContent=message;$("#global-error").hidden=!message;}
async function api(path, data) {
  const response = await fetch(path, {method:data===undefined?"GET":"POST",headers:data===undefined?{}:{"Content-Type":"application/json"},body:data===undefined?undefined:JSON.stringify(data)});
  const result = await response.json();
  if(response.status===401 && path!=="/api/login") showLogin();
  if(!response.ok) throw new Error(result.error || `HTTP ${response.status}`);
  return result;
}
function showLogin(){authenticated=false;$("#login").hidden=false;$("#workspace").hidden=true;icons();}
function config() {
  const form = new FormData($("#audit-form"));
  const data = Object.fromEntries(form.entries());
  data.tests = form.getAll("test"); delete data.test;
  data.require_awsb = form.has("require_awsb") && data.tests.includes("signature_freshness");
  for(const name of ["concurrency","timeout","signature_samples","freshness_samples","freshness_interval","load_max_tokens"]) data[name]=Number(data[name]);
  if(!data.tests.includes("signature_relay")) {data.peer_url="";data.peer_key="";}
  return data;
}
function budget(){
  const data=config();let count=0;
  if(data.tests.includes("conversation")) count+=3;
  if(data.tests.includes("fingerprint") && data.model==="claude-opus-5-5") count+=3;
  if(data.tests.includes("signature_freshness")) count+=data.freshness_models.split(",").filter(v=>v.trim()).length*data.freshness_samples;
  if(data.tests.includes("signature_relay")) count+=data.peer_url?10:4;
  if(data.tests.includes("signature")) count+=data.signature_samples*50;
  if(data.tests.includes("concurrency")) count+=data.concurrency;
  $("#budget").textContent=`最多${count}次生成请求 · 按渠道用量计费`;
  $("#freshness-options").hidden=!data.tests.includes("signature_freshness");
  $("#relay-options").hidden=!data.tests.includes("signature_relay");
  $("#load-options").hidden=!data.tests.includes("concurrency");
}
function findings(summary){
  const seen=new Set();return summary.observations.filter(v=>{if(seen.has(v.message)) return false;seen.add(v.message);return true;}).map(v=>`<div class="finding"><i data-lucide="${v.level==="incomplete"?"circle-help":"triangle-alert"}"></i><span>${esc(v.message)}</span></div>`).join("");
}
function renderOverview(job){
  const s=job.summary;
  if(!s){$("#view-overview").innerHTML=`<div class="working">${esc(labels[job.state])} · 已记录${job.completed_requests}次请求${job.error?`<p class="error">${esc(job.error)}</p>`:""}</div>`;return;}
  let html=`<div class="section-heading"><h2>检测结论</h2>${tag(s.status)}</div><div class="findings">${findings(s)||'<div class="finding"><i data-lucide="circle-check"></i><span>所选检查已完成，未发现已定义的偏差。</span></div>'}</div>`;
  if(s.signature_freshness){
    html+='<h3>实时签名与AWSB标记</h3>' + table(["模型","样本","实时签名","AWSB标记","综合"],s.signature_freshness.models.map(m=>[esc(m.model),`${m.samples}/${m.expected_samples}`,tag(m.freshness),tag(m.awsb),tag(m.status)]));
    html+='<p class="metric-comparison">缺少 f21 的样本无法确认现签。AWSB字段仅作为来源特征，不证明AWS实际调用归属。</p>';
  }
  const cache=s.rows.filter(r=>["conversation","fingerprint"].includes(r.test));
  if(cache.length){html+='<h3>Token与缓存</h3>'+table(["项目","轮次","普通输入","缓存读取","缓存创建","总输入","参照","偏差","命中率","衔接差"],cache.map(r=>[esc(testNames[r.test]),r.round,num(r.result.usage.input_tokens),num(r.result.usage.cache_read_input_tokens),num(r.result.usage.cache_creation_input_tokens),num(r.result.total_input_tokens),num(r.comparison?.expected_total),r.comparison?.ratio!=null?num((r.comparison.ratio-1)*100,2)+"%":"—",r.result.cache_hit_rate!=null?num(r.result.cache_hit_rate*100,2)+"%":"—",num(r.cache_check?.delta)]));}
  if(s.signature){const sig=s.signature;html+='<h3>伪造签名</h3>'+table(["有效来源对照","核心可判定","HTTP400拒绝","完整接受","未评估"],[[`${sig.valid_control_samples}/${sig.expected_samples}`,`${sig.evaluated_core_cases}/${sig.expected_core_cases}`,sig.http_400_rejected_core_cases,sig.accepted_core_cases,sig.unassessed_core_cases]]);const ref=sig.reference?.cases?.append;if(ref)html+=`<p class="metric-comparison">签名末尾追加字符：与可信HTTP400参照一致 ${ref.matched_samples} 次，不同 ${ref.different_samples} 次，未评估 ${ref.unassessed_samples} 次。</p>`;}
  if(s.signature_relay){html+='<h3>工具续接与原题绑定</h3>'+table(["方向","案例","结果","原样对照"],s.signature_relay.cases.map(c=>[esc(c.direction),esc(relayNames[c.case]||c.case),tag(c.outcome),tag(c.controls_valid?"pass":"unknown")]));}
  if(s.concurrency){const m=s.concurrency.metrics;html+='<h3>并发性能</h3>'+table(["指标","P50 / 秒","P90 / 秒","P95 / 秒"],[["首字延迟",num(m.first_text_seconds_success.p50,3),num(m.first_text_seconds_success.p90,3),num(m.first_text_seconds_success.p95,3)],["完整响应",num(m.response_seconds_success.p50,3),num(m.response_seconds_success.p90,3),num(m.response_seconds_success.p95,3)]]);html+=`<p class="metric-comparison">完整正文 ${m.successful_requests}/${m.requests}；接口完整结束 ${m.protocol_successful_requests}/${m.requests}；已发送在途峰值 ${m.peak_sent_in_flight}；发送跨度 ${num(m.send_spread_seconds,3)} 秒。</p>`;}
  html+='<div class="download-links">'+job.files.map(f=>`<a href="/api/jobs/${job.id}/files/${encodeURIComponent(f)}">${esc(f)}</a>`).join("")+'</div>';
  $("#view-overview").innerHTML=html;
}
function renderSignatures(job){
  const rows=job.summary?.rows||[];let html="";
  const fresh=rows.filter(r=>r.test==="signature_freshness");
  if(fresh.length){html+='<h2>签发时间窗口</h2>'+table(["模型 / 轮次","HTTP / 结束","发送时间","收到签名","f21 / 北京时间","f2.f1.f2","f2.f1.f3 / 顶层f3","现签"],fresh.flatMap(r=>{const samples=r.freshness?.signatures||[];return (samples.length?samples:[{}]).map(x=>[esc(`${r.freshness?.request_model} / ${r.round}`),esc(`${r.result.http_status} / ${r.result.stop_reason}`),esc(date(x.sent_at)),esc(date(x.received_at)),x.f2_f1_f21==null?"缺失":esc(x.f2_f1_f21)+"<br>"+esc(date(x.f2_f1_f21)),esc(x.f2_f1_f2),esc(`${x.f2_f1_f3??"—"} / ${x.f3??"—"}`),tag(x.freshness||"unknown")]);}));}
  const sig=rows.filter(r=>r.test==="signature");if(sig.length){html+='<h3>签名变更矩阵</h3>'+table(["序号","场景 / 样本","测试项","HTTP","结果","原样对照"],sig.map(r=>[r.round,esc(`${r.signature.scenario} / ${r.signature.sample}`),esc(r.signature.label),esc(r.result.http_status),tag(r.signature.outcome),tag(r.signature.controls_valid?"pass":"unknown")]));}
  $("#view-signatures").innerHTML=html||'<div class="empty-state"><i data-lucide="shield-question"></i><h2>暂无签名样本</h2></div>';
}
function renderRequests(job){
  $("#view-requests").innerHTML=table(["项目 / 序号","北京时间","HTTP","结束原因","输入合计","首字 / 秒","耗时 / 秒","请求ID"],(job.summary?.rows||[]).map(r=>[esc(`${testNames[r.test]||r.test} / ${r.round}`),esc(date(r.result.started_at)),esc(r.result.http_status),esc(r.result.stop_reason||r.result.error),num(r.result.total_input_tokens),num(r.result.first_text_seconds,3),num(r.result.elapsed_seconds,3),`<span class="mono">${esc(r.result.ids?.["x-request-id"]||r.result.ids?.["request-id"]||r.result.message_id)}</span>`]));
}
function render(job){
  current=job;const s=job.summary;const total=job.plan.target_requests_max+job.plan.reference_requests_max+(job.plan.peer_requests_max||0);
  $("#result-title").textContent=job.label;$("#result-meta").textContent=`${job.config.model} · ${date(job.created_at)} · 服务器2`;
  $("#metric-state").textContent=labels[job.state]||job.state;
  $("#metric-requests").innerHTML=`${job.completed_requests} <small>/ ${total}</small>`;
  $("#metric-freshness").textContent=s?.signature_freshness?labels[s.signature_freshness.status]:"未检测";
  $("#metric-success").textContent=s?.concurrency?num(s.concurrency.metrics.success_rate*100,1)+"%":"未检测";
  $("#progress").max=total||1;$("#progress").value=job.completed_requests;
  $("#cancel").hidden=!['queued','running','cancelling'].includes(job.state);$("#cancel").disabled=job.state==="cancelling";
  $("#download").hidden=!job.files.includes("report.md");
  $("#log-output").textContent=job.log||"等待任务开始";
  renderOverview(job);renderSignatures(job);renderRequests(job);icons();
}
async function history(){
  const response=await api("/api/jobs");historyJobs=response.jobs;
  $("#history-count").textContent=`${historyJobs.length}个任务`;
  $("#history-body").innerHTML=historyJobs.length?historyJobs.map(j=>`<tr class="${j.id===selected?"selected":""}"><td>${esc(j.label)}</td><td class="mono">${esc(j.config.model)}</td><td>${esc(date(j.created_at))}</td><td>${tag(j.state)}</td><td><div class="row-actions"><button class="icon-button" title="查看结果" aria-label="查看结果" data-open="${j.id}"><i data-lucide="arrow-up-right"></i></button><button class="icon-button" title="复用配置" aria-label="复用配置" data-reuse="${j.id}"><i data-lucide="copy"></i></button>${!['queued','running','cancelling'].includes(j.state)?`<button class="icon-button danger" title="删除记录" aria-label="删除记录" data-delete="${j.id}"><i data-lucide="trash-2"></i></button>`:""}</div></td></tr>`).join(""):'<tr><td colspan="5" class="muted-cell">暂无历史任务</td></tr>';
  icons();if(!selected && historyJobs.length)selected=historyJobs[0].id;
}
async function refresh(){if(polling||!authenticated)return;polling=true;try{await history();if(selected){const job=await api(`/api/jobs/${selected}`);if(!current||current.id!==job.id||current.state!==job.state||current.finished_at!==job.finished_at||['running','queued','cancelling'].includes(job.state))render(job);}else if(current){current=null;location.reload();}}catch(e){error(e.message);}finally{polling=false;}}
async function init(){try{await api("/api/me");authenticated=true;$("#login").hidden=true;$("#workspace").hidden=false;await refresh();}catch{showLogin();}icons();budget();}
$("#login-form").addEventListener("submit",async e=>{e.preventDefault();const button=e.submitter;button.disabled=true;try{await api("/api/login",{password:$("#password").value});$("#password").value="";$("#login-error").textContent="";await init();}catch(err){$("#login-error").textContent=err.message;}finally{button.disabled=false;}});
$("#logout").addEventListener("click",async()=>{await api("/api/logout",{});selected=null;current=null;$("#api-key").value="";$("[name=peer_key]").value="";showLogin();});
$("#audit-form").addEventListener("input",budget);
$("#audit-form").addEventListener("submit",async e=>{e.preventDefault();error("");$("#start").disabled=true;try{const job=await api("/api/jobs",config());selected=job.id;$("#api-key").value="";$("[name=peer_key]").value="";await refresh();if(innerWidth<761)$(".results").scrollIntoView({behavior:"smooth"});}catch(err){error(err.message);}finally{$("#start").disabled=false;}});
$("#load-models").addEventListener("click",async()=>{$("#load-models").disabled=true;try{const data=await api("/api/models",config());$("#models").innerHTML=data.models.map(m=>`<option value="${esc(m)}"></option>`).join("");$("#model-status").textContent=data.status==="complete"?`已读取 ${data.models.length} 个模型`:`目录未确认：${data.reason||"返回不完整"}`;}catch(e){$("#model-status").textContent=e.message;}finally{$("#load-models").disabled=false;}});
$("#toggle-key").addEventListener("click",()=>{const show=$("#api-key").type==="password";$("#api-key").type=show?"text":"password";$("#toggle-key").title=show?"隐藏Key":"显示Key";$("#toggle-key").setAttribute("aria-label",$("#toggle-key").title);});
$("#refresh").addEventListener("click",refresh);
$("#cancel").addEventListener("click",async()=>{try{await api(`/api/jobs/${selected}/cancel`,{});await refresh();}catch(e){error(e.message);}});
$("#download").addEventListener("click",()=>{if(selected)location.href=`/api/jobs/${selected}/files/report.md`;});
document.querySelectorAll("[data-tab]").forEach(button=>button.addEventListener("click",()=>{document.querySelectorAll("[data-tab]").forEach(b=>b.classList.toggle("active",b===button));document.querySelectorAll(".view").forEach(v=>v.hidden=v.id!==`view-${button.dataset.tab}`);}));
$("#history-body").addEventListener("click",async e=>{const button=e.target.closest("button");if(!button)return;try{if(button.dataset.open){selected=button.dataset.open;await refresh();}if(button.dataset.reuse){const job=historyJobs.find(j=>j.id===button.dataset.reuse);for(const [name,value] of Object.entries(job.config)){const input=$(`[name="${name}"]`);if(input && name!=="tests"){if(input.type==="checkbox")input.checked=Boolean(value);else input.value=value;}}document.querySelectorAll('[name="test"]').forEach(input=>input.checked=job.config.tests.includes(input.value));$("#api-key").value="";$("[name=peer_key]").value="";budget();$(".configuration").scrollIntoView({behavior:"smooth"});}if(button.dataset.delete && confirm("删除该任务及其报告？")){await api(`/api/jobs/${button.dataset.delete}/delete`,{});if(selected===button.dataset.delete){selected=null;current=null;}await refresh();}}catch(err){error(err.message);}});
setInterval(()=>{if(!document.hidden)refresh();},2500);
init();
