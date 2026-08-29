# 사용자 배포 가이드 (IT 관리자용)

이 앱은 Power BI 데이터를 로컬 캐시(SQLite)로 가져와 보여줍니다.
Power BI 동기화 기능을 쓰려면 **사용자 PC에 아래 1회 설정**이 필요합니다.

## 사전 조건

| 항목 | 내용 |
|---|---|
| OS | Windows 11 |
| Azure CLI | 미설치 시 앱이 자동 감지하여 설치 안내를 표시 |
| az login | 미로그인 시 앱이 자동 감지하여 로그인 안내를 표시 |
| 네트워크 | api.powerbi.com, login.microsoft.com 허용 (사내 프록시 확인 필요) |

## 사용자 안내 (앱 안에서 처리됨)

앱은 시작 시 환경을 자동 진단하여, 문제가 있으면 배너로 알리고 안내 다이얼로그에서
설치/로그인 절차와 명령어(복사 버튼 포함)를 보여줍니다. 사용자가 별도 문서를
읽을 필요 없이 앱 안에서 해결됩니다.

에러 코드별 앱 동작:

| errCode | 의미 | 앱이 보여주는 것 |
|---|---|---|
| azNotFound | Azure CLI 미설치 | winget 설치 명령 + MSI 다운로드 주소 안내 |
| azNotLoggedIn | az 미로그인 | 디바이스 코드 로그인 절차 안내 |
| tokenError | 세션 만료 | 재로그인 절차 안내 |
| apiError | 네트워크/권한 | 네트워크 확인 + 관리자 문의 안내 |
| cacheError | 로컬 캐시 오류 | 앱 재시작 안내 |

## 관리자 수동 안내용 명령

```powershell
# 1. Azure CLI 설치 (관리자 PowerShell)
winget install --id Microsoft.AzureCLI -e

# 2. 로그인 (일반 사용자 권한, 1회)
az login
   (WAM 미지원 시) az login --use-device-code
   (브라우저 방식) az config set core.enable_broker_on_windows=false 후 az login
# → 표시된 코드를 https://login.microsoft.com/device 에 입력 후 회사 계정 로그인

# 3. 확인
az account show
```

## 참고

- 로그인 세션은 보통 수일 유지되며, 만료 시 앱이 tokenError로 안내합니다.
- 동기화 실패해도 마지막으로 성공한 캐시 데이터는 계속 조회됩니다 (오프라인 동작).
- 앱은 az CLI의 토큰을 빌려 쓰므로 클라이언트 시크릿이 코드에 포함되지 않습니다.
