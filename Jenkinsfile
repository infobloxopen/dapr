@Library('jenkins.shared.library') _

pipeline {
  agent {
    label 'ubuntu_docker_label'
  }
  tools {
    go "Go 1.16"
  }
  options {
    checkoutToSubdirectory('src/github.com/infobloxopen/dapr')
  }
  environment {
    GOPATH = "$WORKSPACE"
    DIRECTORY = "src/github.com/infobloxopen/dapr"
    DOCKER_IMAGE = "infoblox/dapr"
    
  }
  stages {
    stage("Setup") {
      steps {
        prepareBuild()
      }
    }
    stage("Test") {
      steps {
        sh "cd $DIRECTORY && make test"
      }
    }
    stage("build-and-archive-binaries-linux-amd64"){
      steps {
        sh "cd $DIRECTORY && make tidy && make release GOOS='linux' GOARCH='amd64' "
      }
    }
    stage("Build-And-Push-Docker") {
      steps {
        dir ("$DIRECTORY") {
          withDockerRegistry([credentialsId: "dockerhub-bloxcicd", url: ""]) {
            sh "make list-of-images"
          }
        }
      }
    }
    stage("Generate-Image-List") {
      steps {
        sh """
          cd $DIRECTORY
          echo "Generating image list manually..."
          GIT_COMMIT_NUMBER=\$(git -C $DIRECTORY rev-parse --short HEAD)
          DAPR_VERSION=v1.0.0-ib-\$GIT_COMMIT_NUMBER
          
          echo "infoblox/dapr:\$DAPR_VERSION" > ${WORKSPACE}/image_list.txt
          echo "infoblox/daprd:\$DAPR_VERSION" >> ${WORKSPACE}/image_list.txt
          echo "infoblox/placement:\$DAPR_VERSION" >> ${WORKSPACE}/image_list.txt
          echo "infoblox/sentry:\$DAPR_VERSION" >> ${WORKSPACE}/image_list.txt
          
          cat ${WORKSPACE}/image_list.txt
        """
      }
    }
  }

  post {
    success {
      script {
        def images = readFile("${WORKSPACE}/image_list.txt").trim()
        echo "Images to finalize: ${images}"
        dir("${WORKSPACE}/${DIRECTORY}") {
          finalizeBuild(images)
        }
      }
    }
    cleanup {
      sh "cd $DIRECTORY && make clean GOOS='linux' GOARCH='amd64'"
    }
  }
}
