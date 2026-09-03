import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';

// 这是不依赖 Go 工具链的插件合约测试，用于验证动态库源码中最容易回归的边界。
const source = await readFile(new URL('../main.go', import.meta.url), 'utf8');
const htmlStart = source.indexOf('const indexHTML = `') + 'const indexHTML = `'.length;
const htmlEnd = source.lastIndexOf('`');
assert.ok(htmlStart > 0 && htmlEnd > htmlStart, '必须存在内嵌资源页面');
const html = source.slice(htmlStart, htmlEnd);
const scriptStart = html.indexOf('<script>') + '<script>'.length;
const scriptEnd = html.indexOf('</script>', scriptStart);
assert.ok(scriptStart > 0 && scriptEnd > scriptStart, '资源页面必须包含脚本');

new Function(html.slice(scriptStart, scriptEnd));

for (const contract of [
  'cliproxy_plugin_init',
  'plugin.register',
  'plugin.reconfigure',
  'management.register',
  'management.handle',
  'Path:        "/dashboard"',
  '/v0/resource/plugins/" + pluginID + "/dashboard',
  'cli-proxy-auth',
  'enc::v1::',
  'cli-proxy-api-webui::secure-storage',
  'managementKey',
  '/v0/management/api-keys',
  '/v0/management/plugins/api-key-manager/config',
  'api_key_metadata',
  'generate-key',
  'var pluginVersion =',
  'management_api',
  'StatusCode',
  'Content-Type',
]) {
  assert.ok(source.includes(contract), `缺少插件合约字段：${contract}`);
}

const digest = createHash('sha256').update('example-api-key', 'utf8').digest('hex');
assert.equal(
  digest,
  '8a7347045a068a4f6975445e94bbcd5247c269dea003fb72f6c3cc2e68c18092',
  '元数据索引必须使用稳定 SHA-256'
);

assert.match(html, /request\(API_KEYS\)/);
assert.match(html, /state\.key=value;try\{await load\(\)/);
assert.match(html, /navigator\.clipboard&&navigator\.clipboard\.writeText/);
assert.match(html, /const copyValue=async value=>/);
assert.match(html, /await copyValue\(value\)/);
assert.match(html, /已生成 API Key，并已复制到剪切板/);
assert.match(html, /copyKey\(item\.index\)/);
assert.match(html, /copy,edit,remove/);
assert.match(html, /crypto\.getRandomValues/);
assert.match(html, /const generateKey=/);
assert.match(html, /id="generate-key"/);
assert.match(html, /method:'PUT'/);
assert.match(html, /method:'PATCH'/);
assert.match(html, /method:'PUT',body:JSON\.stringify\(keys\)/);
assert.match(html, /confirm\('确定删除这个 API Key 吗？'\)/);
assert.doesNotMatch(html, /innerHTML\s*=/, '页面应使用 textContent 写入用户数据');

console.log('contract tests: OK');
