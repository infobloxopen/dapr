@Library('jenkins.shared.library') _

pipeline {
  agent {
    label 'ubuntu_docker_label'
  }
  tools {
    go "Go 1.12"
  }
    options {
        checkoutToSubdirectory('src/github.com/infobloxopen/dapr')
  }
  environment {
    GOPATH = "$WORKSPACE"
    DIRECTORY = "src/github.com/infobloxopen/dapr"
    DOCKER_IMAGE = "smanjunath2"
  }
  stages {
    stage("Setup") {
      steps {
        prepareBuild()
      }
    }
    stage("Lint") {
      steps {
        sh "cd $DIRECTORY && make fmt && git diff --exit-code > /dev/null"
      }
    }
    stage("Test") {
      steps {
        sh "cd $DIRECTORY && make test"
      }
    }
    stage("Build") {
       steps {
        withDockerRegistry([credentialsId: "dockerhub-bloxcicd", url: ""]) {
          sh "cd $DIRECTORY && make docker"
        }
      }
    }
    stage("Push merge") {
       when {
          not { changeRequest() }
          not { buildingTag() }
       }
       steps {
          withDockerRegistry([credentialsId: "dockerhub-bloxcicd", url: ""]) {
             sh 'cd $DIRECTORY && make push-latest USERNAME="Jenkins"'
             sh 'cd $DIRECTORY && make push'
          }
          // AWS_IAM_CI_CD_INFRA
          withAWS(credentials: "CICD_HELM", region: "us-east-1") {
            sh "cd $DIRECTORY && make push-chart"
          }
          dir("${WORKSPACE}/${DIRECTORY}") {
            archiveArtifacts artifacts: 'repo/*.tgz'
            archiveArtifacts artifacts: 'build/build.properties'
          }
       }
    }
    stage("Push Release/Tag") {
       when {
          buildingTag()
       }
       steps {
          withDockerRegistry([credentialsId: "dockerhub-bloxcicd", url: ""]) {
             sh 'cd $DIRECTORY && make push IMAGE_VERSION=${TAG_NAME}'
          }
          // AWS_IAM_CI_CD_INFRA
          withAWS(credentials: "CICD_HELM", region: "us-east-1") {
            sh "cd $DIRECTORY && make push-chart"
          }
          dir("${WORKSPACE}/${DIRECTORY}") {
            archiveArtifacts artifacts: 'repo/*.tgz'
            archiveArtifacts artifacts: 'build/build.properties'
          }
       }
    }
  }
  post {
    success {
      finalizeBuild()
    }
    cleanup {
      sh "cd $DIRECTORY && make clean"
    }
  }
}
