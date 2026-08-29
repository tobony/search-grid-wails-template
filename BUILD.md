# search-grid-template — 빌드/개발 가이드

Wails v2 + React + TypeScript. 타겟: Windows 11 (엔터프라이즈 내부 배포용).

## 개발 환경 (Windows)
- Go 1.21+ / Node.js 18+
- WebView2 런타임: Win11 기본 탑재 (추가 설치 불필요)
- `wails doctor`로 의존성 점검

## 명령어
- 개발(핫리로드): `wails dev`  ← 브라우저에서도 디버깅 가능 (localhost:34115)
- Windows 빌드: `wails build`  → build/bin/search-grid-template.exe
- NSIS 인스톨러: `wails build -nsis`  → build/bin/installer.exe

## Linux에서 크로스컴파일 (프론트엔드 먼저 빌드 필요)
cd frontend && npm install && npm run build
cd .. && GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o build/bin/search-grid-template.exe .
※ 크로스컴파일产物는 NSIS 인스톨러 생성 불가 — 인스톨러는 Windows에서 `wails build -nsis`

## 구조
- main.go: 앱 진입점, 창 설정
- app.go: 백엔드 로직 (프론트에 바인딩됨)
- frontend/wailsjs/go/main/App: Go ↔ 프론트 타입세이프 바인딩 (자동 생성)
