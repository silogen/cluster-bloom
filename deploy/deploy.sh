devbox run build
scp ../dist/bloom plat-test-1:.
scp bloom.1.yaml plat-test-1:bloom.yaml
ssh -t plat-test-1 sudo ./bloom --config bloom.yaml

# kubectl create secret generic argocd-redis -n argocd --from-literal=auth=supersecret
scp ../dist/bloom plat-test-1:additional_node_command.txt plat-test-2:.
ssh plat-test-2 bash additional_node_command.txt
cat bloom.2.yaml | ssh plat-test-2 'cat >> bloom.yaml'
ssh -t plat-test-2 sudo ./bloom --config bloom.yaml
#ssh plat-test-2 echo OK

