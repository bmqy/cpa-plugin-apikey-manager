package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
    void* ptr;
    size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
    uint32_t abi_version;
    void* host_ctx;
    cliproxy_host_call_fn call;
    cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
    uint32_t abi_version;
    cliproxy_plugin_call_fn call;
    cliproxy_plugin_free_fn free_buffer;
    cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;
extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);

static const cliproxy_host_api* stored_host;

static void store_host_api(const cliproxy_host_api* host) {
    stored_host = host;
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unsafe"
)

const (
	abiVersion   uint32 = 1
	schemaVersion uint32 = 3
	pluginID            = "api-key-manager"
	pluginName          = "API Key 管理器"
)

var pluginVersion = "0.1.0"

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type registration struct {
	SchemaVersion uint32                   `json:"schema_version"`
	Metadata      metadata                 `json:"metadata"`
	Capabilities  registrationCapabilities `json:"capabilities"`
}

type metadata struct {
	Name             string        `json:"Name"`
	Version          string        `json:"Version"`
	Author           string        `json:"Author"`
	GitHubRepository string        `json:"GitHubRepository"`
	Logo             string        `json:"Logo"`
	ConfigFields     []configField `json:"ConfigFields"`
}

type configField struct {
	Name        string `json:"Name"`
	Type        string `json:"Type"`
	Description string `json:"Description"`
}

type registrationCapabilities struct {
	ManagementAPI bool `json:"management_api"`
}

type managementRegistration struct {
	Resources []managementResource `json:"resources,omitempty"`
}

type managementResource struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description"`
}

type managementRequest struct {
	Method string      `json:"Method"`
	Path   string      `json:"Path"`
	Header http.Header `json:"Headers"`
	Query  interface{} `json:"Query"`
	Body   []byte      `json:"Body"`
}

type managementResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers"`
	Body       []byte      `json:"Body"`
}

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	C.store_host_api(host)
	plugin.abi_version = C.uint32_t(abiVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}

	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, err := handleMethod(C.GoString(method), requestBytes)
	if err != nil {
		writeResponse(response, errorEnvelope("plugin_error", err.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, length C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
	_ = length
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case "plugin.register", "plugin.reconfigure":
		return okEnvelope(pluginRegistration())
	case "management.register":
		return okEnvelope(managementRegistration{
			Resources: []managementResource{{
				Path:        "/dashboard",
				Menu:        pluginName,
				Description: "在 CLIProxyAPI 管理中心查看、添加、编辑和删除代理 API Key，并维护名称备注。",
			}},
		})
	case "management.handle":
		return handleManagement(request)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: schemaVersion,
		Metadata: metadata{
			Name:             pluginName,
			Version:          pluginVersion,
			Author:           "bmqy",
			GitHubRepository: "https://github.com/bmqy/cpa-plugin-apikey-manager",
			Logo:             "",
			ConfigFields: []configField{{
				Name:        "api_key_metadata",
				Type:        "object",
				Description: "按 API Key 的 SHA-256 索引保存名称和备注，不保存 API Key 明文。",
			}},
		},
		Capabilities: registrationCapabilities{ManagementAPI: true},
	}
}

func handleManagement(raw []byte) ([]byte, error) {
	var request managementRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, fmt.Errorf("解析管理页面请求失败: %w", err)
		}
	}

	path := strings.TrimSpace(request.Path)
	resourcePath := "/v0/resource/plugins/" + pluginID + "/dashboard"
	if path == "/dashboard" || path == resourcePath {
		return okEnvelope(managementResponse{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
			Body:       []byte(indexHTML),
		})
	}
	return okEnvelope(managementResponse{
		StatusCode: http.StatusNotFound,
		Headers:    http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
		Body:       []byte("not found"),
	})
}

func okEnvelope(result interface{}) ([]byte, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>API Key 管理器</title>
<style>
:root{color-scheme:light;--bg:#f5f7fb;--card:#fff;--text:#172033;--muted:#64748b;--line:#e2e8f0;--primary:#2563eb;--danger:#dc2626;--shadow:0 10px 30px rgba(15,23,42,.08)}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{max-width:1180px;margin:0 auto;padding:28px 20px 48px}h1{font-size:25px;margin:0 0 4px}h2{font-size:17px;margin:0 0 14px}.sub{color:var(--muted);margin:0 0 22px}.card{background:var(--card);border:1px solid var(--line);border-radius:14px;box-shadow:var(--shadow);padding:20px;margin:0 0 18px}.auth{display:grid;grid-template-columns:1fr auto;gap:10px;align-items:end}.field{display:flex;flex-direction:column;gap:6px}.field label{font-weight:600}.field small{color:var(--muted)}input,textarea{width:100%;border:1px solid #cbd5e1;border-radius:8px;padding:9px 10px;background:#fff;color:var(--text);font:inherit}textarea{min-height:76px;resize:vertical}button{border:0;border-radius:8px;padding:9px 13px;background:var(--primary);color:#fff;cursor:pointer;font:inherit;font-weight:600}button.secondary{background:#e2e8f0;color:#1e293b}button.danger{background:var(--danger)}button.link{background:transparent;color:var(--primary);padding:3px 6px}.toolbar{display:flex;gap:10px;align-items:center;justify-content:space-between;margin-bottom:14px}.toolbar .actions{display:flex;gap:8px}.status{min-height:22px;color:var(--muted)}.status.error{color:var(--danger)}.status.ok{color:#15803d}.table-wrap{overflow:auto;border:1px solid var(--line);border-radius:10px}table{width:100%;border-collapse:collapse;min-width:760px}th,td{text-align:left;padding:12px 13px;border-bottom:1px solid var(--line);vertical-align:top}th{background:#f8fafc;color:#475569;font-size:12px}tr:last-child td{border-bottom:0}.key{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;word-break:break-all}.muted{color:var(--muted)}.row-actions{display:flex;gap:6px;white-space:nowrap}.modal-backdrop{position:fixed;inset:0;background:rgba(15,23,42,.4);display:none;align-items:center;justify-content:center;padding:18px}.modal-backdrop.show{display:flex}.modal{width:min(560px,100%);background:#fff;border-radius:14px;padding:22px;box-shadow:0 20px 60px rgba(15,23,42,.22)}.modal form{display:grid;gap:14px}.modal-actions{display:flex;justify-content:flex-end;gap:8px}.empty{text-align:center;padding:28px;color:var(--muted)}.hint{font-size:12px;color:var(--muted);margin-top:12px}@media(max-width:700px){main{padding:20px 12px}.auth{grid-template-columns:1fr}.toolbar{align-items:stretch;flex-direction:column}.toolbar .actions{justify-content:flex-end}}
</style>
</head>
<body>
<main>
<h1>API Key 管理器</h1>
<p class="sub">管理 CLIProxyAPI 代理服务认证密钥，并为每个密钥维护名称和备注。</p>
<section class="card">
  <h2>连接</h2>
  <div class="auth"><div class="field"><label for="management-key">Management Key</label><input id="management-key" type="password" autocomplete="off" placeholder="输入 CLIProxyAPI 的管理密钥"><small>会读取同源官方管理面板已记住的会话密钥；本页不会主动持久化你输入的密钥。</small></div><button id="connect">连接并加载</button></div>
  <div id="status" class="status"></div>
</section>
<section class="card">
  <div class="toolbar"><div><h2>代理 API Key</h2><div id="count" class="muted">未连接</div></div><div class="actions"><button id="refresh" class="secondary">刷新</button><button id="add">新增 API Key</button></div></div>
  <div class="field" style="margin-bottom:14px"><label for="search">筛选</label><input id="search" type="search" placeholder="按名称、备注或 API Key 筛选"></div>
  <div class="table-wrap"><table><thead><tr><th>API Key</th><th>名称</th><th>备注</th><th>操作</th></tr></thead><tbody id="rows"><tr><td colspan="4" class="empty">请先连接 CLIProxyAPI。</td></tr></tbody></table></div>
  <div class="hint">名称和备注只保存到本插件配置中，使用 API Key 的 SHA-256 作为索引；API Key 本身由 CLIProxyAPI 管理 API 持久化。</div>
</section>
</main>
<div id="modal-backdrop" class="modal-backdrop"><div class="modal"><h2 id="modal-title">新增 API Key</h2><form id="key-form"><div class="field"><label for="key-value">API Key</label><input id="key-value" required autocomplete="off" spellcheck="false"></div><div class="field"><label for="key-name">名称</label><input id="key-name" maxlength="100" placeholder="例如：个人主账号"></div><div class="field"><label for="key-remark">备注</label><textarea id="key-remark" maxlength="500" placeholder="例如：用于本地开发"></textarea></div><div class="modal-actions"><button type="button" id="cancel" class="secondary">取消</button><button type="submit">保存</button></div></form></div></div>
<script>
(function(){
  'use strict';
  const PLUGIN_CONFIG='/v0/management/plugins/api-key-manager/config';
  const API_KEYS='/v0/management/api-keys';
  const state={key:'',keys:[],metadata:{},editingIndex:-1,visible:{}};
  const $=id=>document.getElementById(id);
  const status=(message,type)=>{const el=$('status');el.textContent=message||'';el.className='status '+(type||'');};
  const AUTH_STORAGE='cli-proxy-auth';
  const ENC_PREFIX='enc::v1::';
  const STORAGE_SALT='cli-proxy-api-webui::secure-storage';
  const decodeOfficialStorage=raw=>{if(!raw||!raw.startsWith(ENC_PREFIX))return raw;try{const encoded=atob(raw.slice(ENC_PREFIX.length));const key=new TextEncoder().encode(STORAGE_SALT+'|'+location.host+'|'+navigator.userAgent);const bytes=new Uint8Array(encoded.length);for(let index=0;index<encoded.length;index++)bytes[index]=encoded.charCodeAt(index)^key[index%key.length];return new TextDecoder().decode(bytes)}catch(_){return ''}};
  const getStoredKey=()=>{try{const raw=localStorage.getItem(AUTH_STORAGE);if(raw){const parsed=JSON.parse(decodeOfficialStorage(raw));const saved=parsed&&typeof parsed==='object'&&parsed.state&&typeof parsed.state==='object'?parsed.state:parsed;const value=saved&&typeof saved.managementKey==='string'?saved.managementKey.trim():'';if(value)return value.replace(/^Bearer\s+/i,'').trim()}}catch(_){}for(const name of ['managementKey','management_key','cpa-management-key']){try{const value=localStorage.getItem(name);if(value&&value.trim())return value.replace(/^Bearer\s+/i,'').trim()}catch(_){}}return '';};
  const request=async(url,options={})=>{if(!state.key)throw new Error('请先输入 Management Key');const headers=new Headers(options.headers||{});headers.set('Authorization','Bearer '+state.key);if(options.body&&!headers.has('Content-Type'))headers.set('Content-Type','application/json');const response=await fetch(url,Object.assign({},options,{headers}));const text=await response.text();let data={};try{data=text?JSON.parse(text):{}}catch(_){data={message:text}}if(!response.ok)throw new Error(data.message||data.error||('请求失败（HTTP '+response.status+'）'));return data;};
  const sha256=async value=>{const bytes=new TextEncoder().encode(value);const digest=await crypto.subtle.digest('SHA-256',bytes);return Array.from(new Uint8Array(digest)).map(item=>item.toString(16).padStart(2,'0')).join('');};
  const loadConfig=async()=>{const data=await request(PLUGIN_CONFIG);return data.api_key_metadata&&typeof data.api_key_metadata==='object'?data.api_key_metadata:{}};
  const saveConfig=async metadata=>{await request(PLUGIN_CONFIG,{method:'PATCH',body:JSON.stringify({api_key_metadata:metadata})});state.metadata=metadata;};
  const load=async()=>{status('正在加载…');const [keyData,metadata]=await Promise.all([request(API_KEYS),loadConfig()]);if(!Array.isArray(keyData['api-keys']))throw new Error('宿主返回的 api-keys 不是字符串数组');state.keys=keyData['api-keys'];state.metadata=metadata;await prepareHashes();render();status('已连接，最后更新：'+new Date().toLocaleTimeString(),'ok');};
  const mask=(value,index)=>state.visible[index]?value:(value.length>8?value.slice(0,4)+'••••'+value.slice(-4):'••••••••');
  const render=()=>{const filter=$('search').value.trim().toLowerCase();const rows=$('rows');rows.textContent='';const items=state.keys.map((key,index)=>({key,index,meta:state.metadata[window.__hashCache[key]||'']||{}})).filter(item=>{const text=(item.key+' '+(item.meta.name||'')+' '+(item.meta.remark||'')).toLowerCase();return !filter||text.includes(filter)});$('count').textContent='共 '+state.keys.length+' 个，当前显示 '+items.length+' 个';if(!items.length){const row=document.createElement('tr');const cell=document.createElement('td');cell.colSpan=4;cell.className='empty';cell.textContent=state.keys.length?'没有匹配项':'暂无 API Key';row.appendChild(cell);rows.appendChild(row);return;}items.forEach(item=>{const row=document.createElement('tr');const keyCell=document.createElement('td');const keyCode=document.createElement('span');keyCode.className='key';keyCode.textContent=mask(item.key,item.index);keyCell.appendChild(keyCode);const toggle=document.createElement('button');toggle.className='link';toggle.textContent=state.visible[item.index]?'隐藏':'显示';toggle.onclick=()=>{state.visible[item.index]=!state.visible[item.index];render();};keyCell.appendChild(toggle);const name=document.createElement('td');name.textContent=item.meta.name||'—';const remark=document.createElement('td');remark.textContent=item.meta.remark||'—';const actions=document.createElement('td');actions.className='row-actions';const edit=document.createElement('button');edit.className='secondary';edit.textContent='编辑';edit.onclick=()=>openModal(item.index);const remove=document.createElement('button');remove.className='danger';remove.textContent='删除';remove.onclick=()=>removeKey(item.index);actions.append(edit,remove);row.append(keyCell,name,remark,actions);rows.appendChild(row);});};
  const openModal=index=>{state.editingIndex=index;$('modal-title').textContent=index<0?'新增 API Key':'编辑 API Key';$('key-value').value=index<0?'':state.keys[index];$('key-name').value=index<0?'':(state.metadata[window.__hashCache[state.keys[index]]||'']?.name||'');$('key-remark').value=index<0?'':(state.metadata[window.__hashCache[state.keys[index]]||'']?.remark||'');$('modal-backdrop').classList.add('show');$('key-value').focus();};
  const closeModal=()=>{$('modal-backdrop').classList.remove('show');state.editingIndex=-1;};
  const removeKey=async index=>{if(!confirm('确定删除这个 API Key 吗？'))return;try{const oldKey=state.keys[index];const keys=state.keys.filter((_,itemIndex)=>itemIndex!==index);const metadata=Object.assign({},state.metadata);delete metadata[await sha256(oldKey)];await request(API_KEYS,{method:'PUT',body:JSON.stringify(keys)});await saveConfig(metadata);state.keys=keys;render();status('API Key 已删除。','ok');}catch(error){status(error.message,'error');}};
  const saveKey=async event=>{event.preventDefault();const value=$('key-value').value.trim();const name=$('key-name').value.trim();const remark=$('key-remark').value.trim();if(!value){status('API Key 不能为空。','error');return;}try{const index=state.editingIndex;const duplicate=state.keys.some((key,itemIndex)=>key===value&&itemIndex!==index);if(duplicate)throw new Error('API Key 已存在。');const keys=state.keys.slice();const metadata=Object.assign({},state.metadata);if(index<0){keys.push(value);metadata[await sha256(value)]={name,remark};}else{const oldKey=keys[index];keys[index]=value;delete metadata[await sha256(oldKey)];metadata[await sha256(value)]={name,remark};}await request(API_KEYS,{method:'PUT',body:JSON.stringify(keys)});await saveConfig(metadata);state.keys=keys;await prepareHashes();closeModal();render();status(index<0?'API Key 已新增。':'API Key 已更新。','ok');}catch(error){status(error.message,'error');}};
  const connect=async()=>{const value=$('management-key').value.trim();if(!value){status('请输入 Management Key。','error');return;}state.key=value;try{await load();}catch(error){status(error.message,'error');}};
  $('management-key').value=getStoredKey();$('connect').onclick=connect;$('refresh').onclick=async()=>{try{await load();}catch(error){status(error.message,'error');}};$('add').onclick=()=>openModal(-1);$('cancel').onclick=closeModal;$('key-form').onsubmit=saveKey;$('search').oninput=render;$('modal-backdrop').onclick=event=>{if(event.target===$('modal-backdrop'))closeModal();};
  window.__hashCache={};
  const prepareHashes=async()=>{for(const key of state.keys)window.__hashCache[key]=await sha256(key);};
  $('add').onclick=()=>openModal(-1);
  state.metadata={};
  connectIfStored();
  async function connectIfStored(){const value=$('management-key').value.trim();if(!value)return;state.key=value;try{await load();}catch(error){status('已读取本地会话，但连接失败：'+error.message,'error');}}
})();
</script>
</body>
</html>`
