# AGENTS.private.md (예시 파일 — 실제 값으로 복사해 쓰기)

# 이 파일을 AGENTS.private.md로 복사한 뒤 각자의 환경으로 채운다.
# AGENTS.private.md는 .gitignore에 등록돼 있어 커밋되지 않는다.

# --- 환경 (로컬 개발 머신) ---
# 호스트: (내부 빌드 서버 호스트명, Debian 12+/Ubuntu 22.04+)
# 프로젝트 절대 경로: /opt/agent/shared/repo/search-grid-template-wails
# Go: /opt/agent/shared/toolchain/go (1.27.0)
# Node: /opt/agent/shared/toolchain/node (22.23.2, npm 10.9.8)
# wails CLI: ~/go-tools/bin/wails (v2.15.0)
# 파일 소유권: 컨테이너 사용자(agent)와 호스트 사용자 공유 — 권한 오류 시 헬메 쪽에서 조정

# --- 개발 서버 접속 ---
# dev 서버 헬메 컨테이너 안에서는 안 됨 (webkit 라이브러리가 빌드 서버 호스트에만 있음)
# 빌드 서버 호스트에서:
#   export PATH=$PATH:/opt/agent/shared/toolchain/go/bin:/opt/agent/shared/toolchain/node/bin:~/go-tools/bin
#   nohup xvfb-run -a wails dev -browser -tags webkit2_41 -devserver "0.0.0.0:34115" > /tmp/wails-dev.log 2>&1 &
# 접속: http://(호스트 내부 IP):34115 (같은 네트워크 팀원도 접속 가능)

# --- Power BI (사내) ---
# 계정: (사내 계정)
# 워크스페이스: (내부 워크스페이스 이름)
# 데이터셋 ID (Push): (GUID)
# 테이블: Docs (id/title/author/category/updated/status)
# 제한: executeQueries 쿼리당 10만 행, rows push 1요청 1만 행(100행 배치 사용 중)

# --- GitHub ---
# private 저장소: (개인계정)/search-grid-template-wails
# push는 헬메 컨테이너 gh 자격증명 사용
