# search-grid-template-wails

Wails v2 + React + TypeScript 데스크톱 앱 (내부용, Win11 타겟, 50명 내외).

Power BI Push 데이터셋을 az CLI 인증으로 읽고/쓰는 검색 그리드 템플릿.
로컬 SQLite 캐시를 둬서 UI는 오프라인 상태에서도 마지막 동기화 데이터로 동작한다.

## 요구 사항

- Go 1.27+ / Node 22+ / Wails CLI v2.15
- Windows 빌드는 NSIS 외 크로스컴파일 가능 (Linux/macOS에서 exe 생성 OK)
- Linux 개발 머신은 webkit2gtk-4.1 패키지 필요

## 개발 서버

    export PATH=$PATH:~/go/bin:~/go-tools/bin
    wails dev -browser -tags webkit2_41 -devserver "0.0.0.0:34115"

- 핫리로드 동작, 브라우저 F12로 DevTools
- Linux 빌드/실행에는 -tags webkit2_41 필수 (webkit2gtk-4.1만 있는 배포판)

## 종료·정리 규칙

- dev 서버는 작업하는 동안만 띄운다. 작업 종료 시 반드시:
      pkill -f "wails dev"; pkill -f Xvfb; sleep 1
- 잔재 확인: pgrep -af "wails|Xvfb" && ss -tlnp | grep 34115
- 남은 프로세스는 소유자 확인 후 제거

## 빌드

- Windows 크로스컴파일 (Linux에서 가능):
      wails build -platform windows/amd64 -ldflags "-s -w" -tags webkit2_41
  -> build/bin/app.exe (약 4.5MB). NSIS 인스톨러(-nsis)는 Windows에서만 가능.

## 아키텍처

- app.go: Wails 바인딩 (GetDocs, SyncFromPowerBI, CheckEnv, AddDoc, DeleteDoc, ExportCSV)
- pbi/: Power BI REST API 클라이언트 (executeQueries, rows push, 401/429 재시도)
- store/: SQLite 로컬 캐시 (modernc.org/sqlite, CGO 불필요)
- diagnose/: 환경 자가 진단 (az 설치/로그인/토큰 확인)
- frontend/: React + TS, Fluent 2 (Microsoft 365) 스타일

## 운영 원칙

- 인증은 앱 등록·시크릿 없이 az CLI 토큰 위임만 사용 (각 사용자 PC에 Azure CLI 설치 전제)
- Power BI Push 데이터셋은 행 단위 삭제가 없어서 삭제는 전체 클리어 후 재업로드 방식
- 토큰 만료·권한 오류는 SetupGuide 다이얼로그로 사용자 언어 안내
- 로컬 캐시가 비어 있을 때만 시작 시 1회 자동 동기화한다(오프라인 우선: 캐시가 있으면 네트워크를 쓰지 않음).
  az 미설치·미로그인이면 배너만 띄우고 시작을 막지 않는다.
- 문서 id는 로컬 캐시의 발급 카운터(`meta.next_id`)로 단조 증가한다. 최대 id 행이 삭제돼도
  재사용하지 않는다(삭제가 전체 클리어+재업로드라 MAX(id)+1만 쓰면 id가 되감겼음).
- 커밋은 사용자 확인 후 진행, Release는 v* 태그 push 시에만

## Private 정보 (로컬 전용)

호스트 IP·경로·계정·데이터셋 ID 같은 내부 운영 정보는 이 저장소에 커밋하지 않는다.
AGENTS.private.example.md를 참고해 AGENTS.private.md를 로컬에 만들어 관리하고,
파일은 .gitignore로 추적에서 제외된다.
