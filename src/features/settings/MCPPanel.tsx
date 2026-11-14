import { useEffect, useState } from "react";
import { useLeaveGuard } from "../../app/nav";
import { StButton, StConfirmButton, StListItem, StNotice, StRow, StSection, StStack, StSwitchItem, StTextField } from "../../design";
import { SettingsController, useMCPStatus } from "./controller";

export function MCPPanel() {
  const { data, error } = useMCPStatus();
  const [port, setPort] = useState<string | null>(null);
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState("");
  const dirty = Boolean(data && port !== null && port !== String(data.port));
  useLeaveGuard(dirty);
  useEffect(() => { setToken(""); }, [data]);
  const configure = async (enabled: boolean, nextPort: number, rotate = false) => {
    if (busy) return;
    setBusy(true);
    setFailed("");
    const saved = await SettingsController.configure(enabled, nextPort, rotate);
    if (saved) { setPort(null); setToken(""); }
    else setFailed("未能更新，请检查端口");
    setBusy(false);
  };
  if (error) return <StNotice tone="danger">{error}</StNotice>;
  if (!data) return null;
  const portText = port ?? String(data.port);
  const validPort = /^\d+$/.test(portText) && Number(portText) >= 1024 && Number(portText) <= 65535;
  return (
    <StStack gap={16}>
      <StSection title="服务" padded={false}>
        <StSwitchItem title="允许本机连接" checked={data.enabled} disabled={busy || dirty} onChange={(enabled) => void configure(enabled, data.port)} />
        <StListItem title="状态" trailing={data.running ? "运行中" : "已停止"} />
        {data.running && <StListItem title="地址" trailing={data.address} />}
      </StSection>
      {(failed || data.error) && <StNotice tone="danger">{failed || data.error}</StNotice>}
      <StSection title="端口">
        <StTextField label="端口" inputMode="numeric" value={portText} onChange={setPort} disabled={busy} width="narrow" error={validPort ? undefined : "请输入 1024 至 65535"} />
      </StSection>
      <StSection title="连接">
        <StStack>
          <StRow>
            <StButton disabled={busy || !data.running} onClick={() => void SettingsController.copyConnection()}>复制连接信息</StButton>
            <StButton disabled={busy || !data.enabled} onClick={async () => {
              if (token) setToken("");
              else setToken((await SettingsController.reveal()) ?? "");
            }} reserveWidthFor={["显示令牌", "隐藏令牌"]}>{token ? "隐藏令牌" : "显示令牌"}</StButton>
            <StButton disabled={busy || !data.enabled} onClick={() => void SettingsController.copyToken()}>复制令牌</StButton>
            <StConfirmButton disabled={busy || dirty || !data.enabled} onConfirm={() => void configure(data.enabled, data.port, true)}>轮换令牌</StConfirmButton>
          </StRow>
          {token && <StTextField label="令牌" value={token} onChange={() => {}} readOnly width="fill" />}
        </StStack>
      </StSection>
      {dirty && <div className="save-bar">
        <StButton disabled={busy} onClick={() => { setPort(null); setFailed(""); }}>放弃</StButton>
        <StButton variant="primary" loading={busy} disabled={!validPort} onClick={() => void configure(data.enabled, Number(portText))}>保存</StButton>
      </div>}
    </StStack>
  );
}
