# Google Play submission notes

## App identity
- Package: `xyz.vandijke.redchef`
- Privacy policy URL: https://redchef.vandijke.xyz/privacy
- Account deletion URL: https://redchef.vandijke.xyz/account/delete

## Data safety (summary)
Collected:
- Account info: email, password hash, role, member flag, created time
- App activity: comments, favourites, tips/purchases (mock), posts/media
- Device/other: session cookie, server logs, IP/user agent via hosting
Shared: none for advertising; processors listed in privacy policy
Deletion: in-app + web URL; immediate; backups may retain until expiry
Encryption in transit: HTTPS only

## Permissions
- INTERNET
- CAMERA (runtime permission, used for uploads)

## Android manifest
- usesCleartextTraffic=false
- allowBackup=false
- fullBackupContent=false
- dataExtractionRules excludes all domains

## Content rating / target audience
- Not child-directed; minimum age 16.