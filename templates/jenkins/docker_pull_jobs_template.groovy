pipeline {
    agent any
    environment {
        PRODUCT_NAME = "{{PRODUCT_NAME}}"
        GIT_REPO = "{{GIT_REPO}}"
        GIT_BRANCH = "{{GIT_BRANCH}}"
        SERVER_IP = '{{SERVER_IP}}'
        SERVER_PORT = {{SERVER_PORT}}
        SSH_AGENT = '{{SSH_AGENT}}'
        REMOTE_PATH = "/var/www/{{PRODUCT_NAME}}"
        TELEGRAM_API = '{{TELEGRAM_API}}'
        TELEGRAM_CHAT_ID = "{{TELEGRAM_CHAT_ID}}"
    }

    stages {
        stage('Notify Start') {
            steps {
                withCredentials([usernamePassword(credentialsId: 'serverbot-telegram', passwordVariable: 'password', usernameVariable: 'username')]) {
                    sh """
                        curl --location '${TELEGRAM_API}/bot${username}:${password}/sendMessage' \
                             --form 'chat_id=${TELEGRAM_CHAT_ID}' \
                             --form 'text=START DEPLOY ${PRODUCT_NAME} FROM ${GIT_REPO} ON ${GIT_BRANCH}'
                    """
                }
            }
        }

        stage('Deploy via SSH') {
            steps {
                script {
                    sshagent(["${SSH_AGENT}"]) {
                        sh """
                            ssh -o StrictHostKeyChecking=no root@${SERVER_IP} -p ${SERVER_PORT} '
                                echo "Deploying ${PRODUCT_NAME}..." &&
                                cd ${REMOTE_PATH} &&
                                git checkout ${GIT_BRANCH} &&
                                git fetch origin &&
                                git merge -X theirs origin/${GIT_BRANCH} --no-edit &&
                                docker compose down && docker compose up -d
                            '
                        """
                    }
                }
            }
        }
    }

    post {
        success {
            withCredentials([usernamePassword(credentialsId: 'serverbot-telegram', passwordVariable: 'password', usernameVariable: 'username')]) {
                sh """
                    curl --location '${TELEGRAM_API}/bot${username}:${password}/sendMessage' \
                         --form 'chat_id=${TELEGRAM_CHAT_ID}' \
                         --form 'text=✅ DEPLOY SUCCESS ${PRODUCT_NAME}'
                """
            }
        }
        failure {
            withCredentials([usernamePassword(credentialsId: 'serverbot-telegram', passwordVariable: 'password', usernameVariable: 'username')]) {
                sh """
                    curl --location '${TELEGRAM_API}/bot${username}:${password}/sendMessage' \
                         --form 'chat_id=${TELEGRAM_CHAT_ID}' \
                         --form 'text=❌ DEPLOY FAILED ${PRODUCT_NAME}'
                """
            }
        }
    }
}
