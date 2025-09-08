# gcs-csi
    1、在k8s上通过gcsfuse客户端将gcs挂载给业务pod使用
    2、支持gcp账户密钥和gcp workload identiy两种认证方式
    3、支持静态pv/pvc、动态sc/pvc挂载

# 编译
```
gcs csi driver:
    GOOS=linux GOOS=linux GOARCH=amd64 go build -o bin/driver cmd/main.go

csifuse-proxy:
    GOOS=linux GOOS=linux GOARCH=amd64 go build -o pkg/csifuse-proxy/bin/csifuse-proxy pkg/csifuse-proxy/server/main.go
```


# 镜像构建
```
cd dockerfile
docker build -t gcscsi:v1.0 .
```


# gcs-csi部署
kubectl apply -k deploy/overlays/stable

# 账户密钥认证方式配置和挂载实例
```
1、创建gcp服务账户
   gcloud iam service-accounts create gcscsi-driver \
    --description="gcsfuse csi service account" \
    --display-name="gcscsi-driver"

2、为该服务账户分配权限
   gcloud projects add-iam-policy-binding my-project \
    --member="serviceAccount:gcscsi-driver@my-project.iam.gserviceaccount.com" \
    --role="roles/storage.objectAdmin"

3、创建服务账户密钥
   gcloud iam service-accounts keys create keys.json --iam-account=gcscsi-driver@my-project.iam.gserviceaccount.com

4、为pv创建k8s secret，将3中的账户密钥keys.json注入k8s pv secret中
   kubectl create secret generic csi-gcs-secret --from-file=key=keys.json

5、部署deployment pod测试验证(pv.yaml里根据业务实际情况修改bucket和挂载optoins)
    静态key方式： 
		kubectl apply -k examples/static-key/
   
    静态wi方式：  
		kubectl apply -k examples/static-wi/
   
    动态key方式： 
		kubectl apply -f examples/dy-key/sc.yaml
		kubectl apply -f examples/dy-key/pvc.yaml
		kubectl apply -f examples/dy-key/deployment.yaml
   
    动态mi方式：
		kubectl apply -f examples/dy-wi/sc.yaml
                kubectl apply -f examples/dy-wi/pvc.yaml
                kubectl apply -f examples/dy-wi/deployment.yaml 
		
```


# Workload Identity Federation认证方式配置和挂载实例
```
参考： https://cloud.google.com/iam/docs/workload-identity-federation-with-kubernetes?hl=zh-cn#deploy
1、获取k8s集群openid-configuration和jwks
    kubectl get --raw /.well-known/openid-configuration > openid-configuration
    kubectl get --raw /openid/v1/jwks > jwks
2、创建公开只读可访问的gcs bucket，并创建子目录.well-known，如该bucket为gcscsi-oidc    
3、修改openid-configuration里的issuer和jwks_uri字段值为gcs可公开访问的只读桶路径，一个完整openid-configuration的实例内容为：
{
    "issuer": "https://storage.googleapis.com/gcscsi-oidc",
    "jwks_uri": "https://storage.googleapis.com/gcscsi-oidc/jwks",
    "authorization_endpoint": "urn:kubernetes:programmatic_authorization",
    "response_types_supported": [
        "id_token"
    ],
    "subject_types_supported": [
        "public"
    ],
    "id_token_signing_alg_values_supported": [
        "RS256"
    ],
    "claims_supported": [
        "sub",
        "iss"
    ]
}
4、将上述openid-configuration上传到gcscsi-oidc桶的.well-known子目录下，将jwks文件上传到gcscsi-oidc根目录下

5、修改k8s apiserver配置，增加下面两项并重启apiserver：
    --service-account-issuer=https://storage.googleapis.com/gcscsi-oidc
    --api-audiences=sts.googleapis.com

6、gcp IAM的工作负载身份联合创建身份池，并向该身份池添加OIDC身份提供方
    gcloud iam workload-identity-pools create gcscsi-pool \
    --location="global" \
    --description="gcsfuse csi pool" \
    --display-name="gcscsi-pool"

    gcloud iam workload-identity-pools providers create-oidc gcscsi-oidc \
    --location="global" \
    --workload-identity-pool="gcscsi-pool" \
    --issuer-uri="https://storage.googleapis.com/gcscsi-oidc" \
    --attribute-mapping="google.subject=assertion.sub"

7、为gcs-csi driver部署在k8s中的所需serviceaccount授予IAM和gcs访问权限，如gcs-drver部署在gcscsi namespace下：
   7.1 kubectl create serviceaccount gcscsi-sa -n gcscsi
   7.2 修改deploy下所有namespace和serviceaccount字段为gcscsi和gcscsi-sa
   7.3 为sa授权iam访问权限
    gcloud projects add-iam-policy-binding projects/PROJECT_ID \
    --role=roles/container.clusterViewer \
    --role=roles/storage.Admin \
    --member=principal://iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL_ID/subject/MAPPED_SUBJECT \
    --condition=None

    注：在gcp测试账号下，生产账号联系运维获取操作
         PROJECT_ID = abc-poc-test
         PROJECT_NUMBER = 362600025029
         POOL_ID = gcscsi-pool
         MAPPED_SUBJECT = system:serviceaccount:gcscsi:gcscsi-sa

9、 为某个需要被挂载的gcs桶授予账号：
    principal://iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL_ID/subject/MAPPED_SUBJECT 访问gcs权限: Storage Admin

10、下载认证信息
    gcloud iam workload-identity-pools create-cred-config \
    projects/362600025029/locations/global/workloadIdentityPools/gcscsi-pool/providers/gcscsi-oidc \
    --credential-source-file=/var/run/service-account/token \
    --credential-source-type=text \
    --output-file=cred.json    

11、将cred.json通过configmap提供给gcs-csi driver:
    kubectl create configmap gcp-cred \
    --from-file cred.json \
    --namespace gcscsi

12、修改deploy/base/daemonset.yaml里的serviceAccountToken audience字段为：
    https://iam.googleapis.com/projects/362600025029/locations/global/workloadIdentityPools/gcscsi-pool/providers/gcscsi-oidc

13、部署gcs-csi driver: 
    kubectl apply -k deploy/overlays/stable

14、部署deployment pod测试验证(pv.yaml里根据业务实际情况修改bucket和挂载optoins)
    kubectl apply -k examples/static-nokey/
```
