# iac-pulumi
NEU-csye6225 assignment

### command note

- change stack
  - $env:AWS_PROFILE = "demo"
- show current aws profile 
  - aws configure list
- show all aws profiles
  - aws configure list-profiles
- show pulumi stack config
  - pulumi config
- GCP command login
  - gcloud auth login
  - gcloud auth application-default login
- check dynamodb
```powershell
  aws dynamodb scan --table-name your-table-name
  ```
- import ssl certificate
```powershell
aws acm import-certificate `
--certificate fileb://path_to_your_certificate_file.crt `
--private-key fileb://path_to_your_private_key_file.key `
--certificate-chain fileb://path_to_your_ca_bundle_file.crt
```



  



