import { useState } from 'react';

export interface SyncResult {
  ok: boolean;
  count?: number;
  syncedAt?: string;
  errCode?: string;
  errMsg?: string;
  errHint?: string;
}

interface Guide {
  title: string;
  steps: { text?: string; cmd?: string; note?: string }[];
  note?: string;
  isOk?: boolean;
}

// 공통: 로그인 방식 3종 (WAM -> device-code -> 브라우저 순서)
const LOGIN_METHODS = [
  {
    text: '방법 1 (권장): Windows 계정으로 바로 로그인 (WAM). 아래 명령을 실행하면 계정 선택 창이 뜹니다. 회사 계정을 선택하면 끝납니다:',
    cmd: 'az login',
    note: '이미 Windows에 로그인된 회사 계정을 그대로 사용합니다. MFA·보안 정책도 Windows와 동일하게 적용됩니다.',
  },
  {
    text: '방법 2: 디바이스 코드로 로그인 (방법 1이 안 되면). 아래 명령을 실행하면 코드가 표시됩니다. 코드를 복사하세요:',
    cmd: 'az login --use-device-code',
    note: '복사한 코드를 https://login.microsoft.com/device 에 입력하고 회사 계정으로 로그인하세요.',
  },
  {
    text: '방법 3: 브라우저 창으로 로그인 (방법 1, 2 모두 안 되면). 먼저 아래 명령으로 Windows 계정 브로커를 끄세요:',
    cmd: 'az config set core.enable_broker_on_windows=false',
    note: '이후 터미널에서 az login을 다시 실행하면 브라우저 창이 열립니다. 브라우저에서 회사 계정으로 로그인하면 완료됩니다.',
  },
];

const GUIDES: Record<string, Guide> = {
  azNotFound: {
    title: 'Azure CLI 설치가 필요합니다',
    steps: [
      { text: '1. PowerShell을 관리자 권한으로 실행하세요 (시작 버튼 → "powershell" 검색 → 마우스 오른쪽 → 관리자 권한 실행)' },
      { text: '2. 아래 명령을 복사해서 붙여넣고 실행하세요 (설치에 3~5분 걸립니다):', cmd: 'winget install --id Microsoft.AzureCLI -e' },
      { text: '3. winget이 없거나 실패하면 아래 주소에서 "Azure CLI 다운로드"를 눌러 MSI를 직접 설치하세요.', note: 'https://aka.ms/installazurecliwindows' },
      { text: '4. 설치 후 이 앱을 다시 시작하고 동기화를 다시 눌러주세요.' },
    ],
    note: '설치가 끝나면 앱을 재시작해야 새로 설치된 프로그램을 인식합니다.',
  },
  azNotLoggedIn: {
    title: 'Azure CLI 로그인이 필요합니다',
    steps: [
      { text: '1. 명령 프롬프트(cmd) 또는 PowerShell을 여세요.' },
      { text: '2. 아래 3가지 방법 중 순서대로 시도하세요. 첫 번째(방법 1)가 대부분 바로 됩니다.' },
      ...LOGIN_METHODS,
      { text: '로그인이 끝나면 이 앱에서 동기화를 다시 누르면 됩니다.' },
    ],
    note: '로그인은 보통 며칠간 유지됩니다. 세션이 만료되면 같은 방법으로 다시 로그인하세요.',
  },
  tokenError: {
    title: '로그인 세션이 만료되었습니다',
    steps: [
      { text: '1. 명령 프롬프트(cmd) 또는 PowerShell을 여세요.' },
      { text: '2. 아래 3가지 방법 중 순서대로 시도하세요.' },
      ...LOGIN_METHODS,
      { text: '로그인이 끝나면 이 앱에서 동기화를 다시 눌러주세요.' },
    ],
  },
  forbiddenError: {
    title: '데이터셋 접근 권한이 없습니다',
    steps: [
      { text: '이 데이터셋(Power BI Release Plan)에 대한 읽기 권한이 없는 계정입니다.' },
      { text: '1. Power BI 웹(https://app.powerbi.com)에서 동일한 데이터셋이 보이는지 확인해 주세요.' },
      { text: '2. 웹에서도 안 보이면 Power BI 관리자에게 접근 권한을 요청해 주세요.' },
    ],
    note: '권한이 부여되면 앱에서 동기화를 다시 누르면 됩니다.',
  },
  apiError: {
    title: 'Power BI에 연결하지 못했습니다',
    steps: [
      { text: '1. 네트워크 연결(사내망/VPN)을 확인해 주세요.' },
      { text: '2. Power BI 웹(https://app.powerbi.com)에 접속이 되는지 확인해 주세요. 웹이 되는데 앱만 안 되면 아래에 표시된 오류 내용을 관리자에게 전달해 주세요.' },
      { text: '3. 문제가 계속되면 관리자에게 문의해 주세요.' },
    ],
    note: '동기화에 실패해도 마지막에 성공한 데이터는 계속 볼 수 있습니다.',
  },
  envOk: {
    title: '✅ 환경 진단 완료 — 정상 동작 중입니다',
    isOk: true,
    steps: [
      { text: 'Azure CLI 설치와 로그인 상태가 모두 정상입니다.' },
      { text: 'Power BI 동기화 버튼을 누르면 최신 데이터를 가져올 수 있습니다.' },
      { text: '이 안내가 보이는 동안에도 마지막으로 동기화된 데이터는 계속 볼 수 있습니다.' },
    ],
  },
  cacheError: {
    title: '로컬 데이터 저장소 오류',
    steps: [
      { text: '앱을 완전히 종료한 뒤 다시 시작해 주세요.' },
      { text: '반복되면 관리자에게 오류 메시지를 전달해 주세요.' },
    ],
  },
};

export function SetupGuideDialog({ code, msg, hint, onClose }: {
  code: string; msg?: string; hint?: string; onClose: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const guide = GUIDES[code];
  if (!guide) return null;

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch { /* clipboard may be blocked */ }
  };

  return (
    <div className="dialog-backdrop" onClick={onClose}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <div className={guide.isOk ? "dialog-title dialog-title-ok" : "dialog-title"}>{guide.title}</div>
        {msg && <div className="dialog-err">{msg}</div>}
        <ol className="dialog-steps">
          {guide.steps.map((s, i) => (
            <li key={i}>
              {s.cmd ? (
                <div className="cmdline">
                  <code>{s.cmd}</code>
                  <button className="copybtn" onClick={() => { void copy(s.cmd!); }}>
                    {copied ? '복사됨' : '복사'}
                  </button>
                </div>
              ) : (
                <span>{s.text}</span>
              )}
              {s.note && <div className="cmdnote">{s.note}</div>}
            </li>
          ))}
        </ol>
        {guide.note && <div className="dialog-note">💡 {guide.note}</div>}
        {hint && <div className="dialog-hint">명령: <code>{hint}</code></div>}
        <div className="dialog-actions">
          <button className="cmdbtn primary" onClick={onClose}>확인</button>
        </div>
      </div>
    </div>
  );
}
