pipeline {
    agent any
    environment {
        REGISTRY = "registry2.dotb.cloud"
        IMAGE = "main3.$BUILD_NUMBER"
        PRODUCT_NAME = "{{PRODUCT_NAME}}"
        GIT_REPO = "{{GIT_REPO}}"
        GIT_BRANCH = "{{GIT_BRANCH}}"
        SERVER_IP = '{{SERVER_IP}}'
        SERVER_PORT = {{SERVER_PORT}}
        SSH_AGENT = '{{SSH_AGENT}}'
        REMOTE_PATH = "/var/www/${PRODUCT_NAME}"
        HELM_DEPLOY = "{{HELM_DEPLOY}}"
        TELEGRAM_API = '{{TELEGRAM_API}}'
        TELEGRAM_CHAT_ID = "{{TELEGRAM_CHAT_ID}}"
        KUBECONFIG_PATH = "{{KUBECONFIG_PATH}}"
    }

    stages {
        // stage('Notify Start') {
        //     steps {
        //         withCredentials([usernamePassword(credentialsId: 'serverbot-telegram', passwordVariable: 'password', usernameVariable: 'username')]) {
        //             sh """
        //                 curl --location '${TELEGRAM_API}/bot${username}:${password}/sendMessage' \
        //                      --form 'chat_id=${TELEGRAM_CHAT_ID}' \
        //                      --form 'text=START DEPLOY ${PRODUCT_NAME} FROM ${GIT_REPO} ON ${GIT_BRANCH}'
        //             """
        //         }
        //     }
        // }

        stage('Clone and update git repo') {
          steps {
                script {
                    git([url: "${env.GIT_REPO}", branch: "${env.GIT_BRANCH}", credentialsId: 'gitea_mrtux'])
          }
        }

        stage('Build image') {
           steps{
                script {
                    docker.build "${REGISTRY}/${PRODUCT_NAME}:${IMAGE}"
                }
            }
        }

        stage('Push image'){
          steps{
                script {
                    docker.withRegistry("https://${REGISTRY}", 'docker_registry2') {
                        docker.image("${REGISTRY}/${PRODUCT_NAME}:${IMAGE}").push()
                    }
                }
            }
        }

          stage('Deploy') {
            steps {
                script {
                    sshagent(["${env.SSH_AGENT}"]) {
                        sh """
                            ssh -o StrictHostKeyChecking=no root@${env.SERVER_IP} -p ${env.SERVER_PORT} "cd /root/k8s/DotB/${PRODUCT_NAME} && export KUBECONFIG=${env.KUBECONFIG_PATH} && helm upgrade --set image=${REGISTRY}/${PRODUCT_NAME}:${IMAGE} ${env.HELM_DEPLOY} --wait;"
                        """
                    }
                }
            }
        }
    }

    // post {
    //     success {
    //         withCredentials([usernamePassword(credentialsId: 'serverbot-telegram', passwordVariable: 'password', usernameVariable: 'username')]) {
    //             sh """
    //                 curl --location '${TELEGRAM_API}/bot${username}:${password}/sendMessage' \
    //                      --form 'chat_id=${TELEGRAM_CHAT_ID}' \
    //                      --form 'text=✅ DEPLOY SUCCESS ${PRODUCT_NAME}'
    //             """
    //         }
    //     }
    //     failure {
    //         withCredentials([usernamePassword(credentialsId: 'serverbot-telegram', passwordVariable: 'password', usernameVariable: 'username')]) {
    //             sh """
    //                 curl --location '${TELEGRAM_API}/bot${username}:${password}/sendMessage' \
    //                      --form 'chat_id=${TELEGRAM_CHAT_ID}' \
    //                      --form 'text=❌ DEPLOY FAILED ${PRODUCT_NAME}'
    //             """
    //         }
    //     }
    // }
}
