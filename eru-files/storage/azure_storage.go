package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	eruaes "github.com/eru-tech/eru/eru-crypto/aes"
	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	"github.com/segmentio/ksuid"
)

//const iv = "0123456789ABCDEF"

type AzureStorage struct {
	Storage
	Region         string `json:"region"`
	BucketName     string `json:"bucket_name" eru:"required"`
	Authentication string `json:"authentication" eru:"required"`
	SubscriptionId string `json:"subscription_id" eru:"required"`
	TenantId       string `json:"tenant_id" eru:"required"`
	ClientId       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	ResourceGroup  string `json:"resource_group" eru:"required"`
	session        azcore.TokenCredential
}

func (azureStorage *AzureStorage) GetAttribute(attributeName string) (attributeValue interface{}, err error) {
	switch attributeName {
	case "storage_name":
		return azureStorage.StorageName, nil
	case "storage_type":
		return azureStorage.StorageType, nil
	case "key_pair":
		return azureStorage.KeyPair, nil
	case "key_id":
		return azureStorage.KmsId, nil
	case "bucket_name":
		return azureStorage.BucketName, nil
	case "subscription_id":
		return azureStorage.SubscriptionId, nil
	case "resource_group":
		return azureStorage.ResourceGroup, nil
	case "region":
		return azureStorage.Region, nil
	case "authentication":
		return azureStorage.Authentication, nil
	case "client_id":
		return azureStorage.ClientId, nil
	case "client_secret":
		return azureStorage.ClientSecret, nil
	case "tenant_id":
		return azureStorage.TenantId, nil
	case "session":
		return azureStorage.session, nil
	default:
		return nil, errors.New("attribute not found")
	}
}

func (azureStorage *AzureStorage) getServiceClient(connectionString string) (serviceClient *azblob.Client) {
	connectionString = azureStorage.Authentication
	serviceClient, err := azblob.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		logs.WithContext(context.Background()).Debug("Failed to connect with azure")
		return
	}
	return serviceClient
}

func (azureStorage *AzureStorage) getAzureIdentityCredential(connectionString string) (identityCred *azidentity.ClientSecretCredential) {
	connectionString = azureStorage.Authentication

	return nil
}

func (azureStorage *AzureStorage) ContainerExists(ctx context.Context, containerName string) (exists bool, err error) {
	logs.WithContext(ctx).Debug("Checking BucketExists- Start")
	connectionString := azureStorage.Authentication
	serviceClient := azureStorage.getServiceClient(connectionString)
	_, err = serviceClient.ServiceClient().NewContainerClient(containerName).GetProperties(ctx, nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (azureStorage *AzureStorage) CreateContainer(ctx context.Context, containerName string) (*container.Client, container.CreateResponse, error) {
	connectionString := azureStorage.Authentication

	ctx = context.TODO()
	logs.WithContext(ctx).Debug("Create Container - Start")

	serviceClient := azureStorage.getServiceClient(connectionString)
	containerClient := serviceClient.ServiceClient().NewContainerClient(containerName)
	slog.Info(containerClient.URL())
	createContainerResponse, err := containerClient.Create(ctx, nil)

	slog.Info("", createContainerResponse)
	if err != nil {
		logs.WithContext(ctx).Debug(fmt.Sprintf("Failed to create container: %s , Error: %s", containerName, err.Error()))
		slog.Info("", createContainerResponse)
		return containerClient, container.CreateResponse{}, err
	}

	return containerClient, createContainerResponse, nil
}
func (azureStorage *AzureStorage) Init(ctx context.Context) (err error) {
	logs.WithContext(ctx).Debug("Init - Start")
	var cred azcore.TokenCredential
	armstorage.NewAccountsClient(azureStorage.SubscriptionId, cred, nil)

	if azureStorage.Authentication == AuthTypeSecret {
		cred, err = azidentity.NewClientSecretCredential(azureStorage.TenantId, azureStorage.ClientId, azureStorage.ClientSecret, nil)
		if err != nil {
			logs.WithContext(ctx).Error(fmt.Sprintf("failed to create client secret credential: %s", err.Error()))
			return err
		}
	} else if azureStorage.Authentication == AuthTypeIAM {
		logs.WithContext(ctx).Info("connecting Azure storage with IAM role")
		cred, err = azidentity.NewDefaultAzureCredential(nil)
		if err == nil {
			// Test token acquisition to validate
			_, err = cred.GetToken(ctx, policy.TokenRequestOptions{
				Scopes: []string{"https://management.azure.com/.default"},
			})
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprintf("failed to get token: %s", err.Error()))
				return err
			}
		}
	}
	azureStorage.session = cred
	return nil
}

func (azureStorage *AzureStorage) UploadFileB64(ctx context.Context, file []byte, fileName string, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {

	connectionString := azureStorage.Authentication

	containerName := folderPath
	slog.Info(containerName)
	fmt.Printf(containerName)
	ctx = context.TODO()
	enc := ""
	finalFileName := ""
	docId = ksuid.New().String()
	logs.WithContext(ctx).Debug("UploadFileB64 - Start")

	serviceClient := azureStorage.getServiceClient(connectionString)

	isContainerExist, err := azureStorage.ContainerExists(ctx, containerName)
	slog.Info("is Container present?", isContainerExist)
	fmt.Println(isContainerExist)
	if isContainerExist == true {
		if azureStorage.EncryptFiles {
			enc = ".enc"
			var fileKey []byte
			finalFileName = fmt.Sprintf("%s_%s%s", docId, fileName, enc)

			file, fileKey, err = azureStorage.encrypt(ctx, file, keyName)
			fmt.Println("File Key", string(fileKey))

			if err != nil {
				logs.WithContext(ctx).Debug(fmt.Sprintf("Failed To encrypt file %s, error: %s", fileName, err.Error()))
				return docId, err
			}
			file = append(file, []byte("___eru___")...)
			file = append(file, fileKey...)
			fmt.Println(string(file))

		} else {
			finalFileName = fmt.Sprintf("%s_%s", docId, fileName)
		}

		containerClient := serviceClient.ServiceClient().NewContainerClient(containerName)
		slog.Info(containerClient.URL())

		fmt.Println("docType", docType)

		fmt.Println("docId", docId)

		fmt.Println("fileName", fileName)

		fmt.Println("enc", enc)

		fmt.Println("Final File Name:", finalFileName)
		blobClient := containerClient.NewBlockBlobClient(finalFileName)

		reader := bytes.NewReader(file)

		_, err = blobClient.UploadStream(ctx, reader, nil)
		if err != nil {
			logs.WithContext(ctx).Debug(fmt.Sprintf("Failed To Upload File on %s , error: %s", containerName, err.Error()))
			return
		} else {
			logs.WithContext(ctx).Debug(fmt.Sprintf("File Uploaded Successfully on %s , status: %s", containerName, "success"))
		}
		return
	} else {
		logs.WithContext(ctx).Debug("Container is not present, Container creation started...")
		containerClient, createContainerResponse, _ := azureStorage.CreateContainer(ctx, containerName)
		slog.Info("New Container Responce", createContainerResponse)
		fmt.Printf("New Container Responce", createContainerResponse)
		if createContainerResponse.Date != nil {

			if azureStorage.EncryptFiles {
				enc = ".enc"
				var fileKey []byte
				finalFileName = fmt.Sprintf("%s_%s%s", docId, fileName, enc)
				file, fileKey, err = azureStorage.encrypt(ctx, file, keyName)
				fmt.Println("File Key", string(fileKey))

				if err != nil {
					logs.WithContext(ctx).Debug(fmt.Sprintf("Failed To encrypt file %s, error: %s", fileName, err.Error()))
					return docId, err
				}
				file = append(file, []byte("___eru___")...)
				file = append(file, fileKey...)
				fmt.Println(string(file))
			} else {
				finalFileName = fmt.Sprintf("%s_%s", docId, fileName)
			}

			slog.Info(containerClient.URL())

			slog.Info("Final File Name", finalFileName)
			blobClient := containerClient.NewBlockBlobClient(finalFileName)

			reader := bytes.NewReader(file)

			_, err = blobClient.UploadStream(ctx, reader, nil)
			if err != nil {
				logs.WithContext(ctx).Debug(fmt.Sprintf("Failed To Upload File on %s , error: %s", containerName, err.Error()))
				return docId, err
			} else {
				logs.WithContext(ctx).Debug(fmt.Sprintf("File Uploaded Successfully on %s , status: %s", containerName, "success"))
			}
			return docId, err
		}

	}
	return docId, err
}

func (azureStorage *AzureStorage) UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {
	logs.WithContext(ctx).Debug("UploadFile - Start")
	connectionString := azureStorage.Authentication
	containerName := folderPath
	enc := ".enc"
	finalFileName := ""
	fileKey := []byte{}
	docId = ksuid.New().String()

	ctx = context.TODO()

	serviceClient := azureStorage.getServiceClient(connectionString)

	containerClient := serviceClient.ServiceClient().NewContainerClient(containerName)
	isContainerExist, err := azureStorage.ContainerExists(ctx, containerName)
	docId = ksuid.New().String()
	if isContainerExist == true {

		file, err := io.ReadAll(file)

		if err != nil {
			slog.Info("Failed to read File content")
			logs.WithContext(ctx).Debug("Failed to read File content")
			return docId, err
		}
		if azureStorage.EncryptFiles {

			finalFileName = fmt.Sprintf("%s_%s%s", docId, header.Filename, enc)
			file, fileKey, err = azureStorage.encrypt(ctx, file, keyName)
			if err != nil {
				logs.WithContext(ctx).Debug("Failed to encrypt file")
				return docId, err
			}
			file = append(file, []byte("___eru___")...)
			file = append(file, fileKey...)
		} else {
			finalFileName = fmt.Sprintf("%s_%s", docId, header.Filename)
		}
		blobClient := containerClient.NewBlockBlobClient(finalFileName)

		reader := bytes.NewReader(file)
		_, err = blobClient.UploadStream(ctx, reader, nil)
		if err != nil {
			slog.Info("Failed To Upload File")
			logs.WithContext(ctx).Debug("Failed To Upload File")
			return docId, err
		} else {
			slog.Info("File Uploaded Successfully")
			logs.WithContext(ctx).Debug("File Uploaded Successfully")
			return docId, err
		}
	} else {
		logs.WithContext(ctx).Debug("Container is not present, Container creation started...")
		containerClient, createContainerResponse, _ := azureStorage.CreateContainer(ctx, containerName)
		slog.Info("New Container Responce", createContainerResponse)
		fmt.Printf("New Container Responce", createContainerResponse)
		if createContainerResponse.Date != nil {
			file, err := io.ReadAll(file)
			if err != nil {
				slog.Info("Failed to read File content")
				logs.WithContext(ctx).Debug("Failed to read File content")
				return docId, err
			}
			if azureStorage.EncryptFiles {
				finalFileName = fmt.Sprintf("%s_%s%s", docId, header.Filename, enc)
				file, fileKey, err = azureStorage.encrypt(ctx, file, keyName)
				if err != nil {
					logs.WithContext(ctx).Debug("Failed to encrypt file")
					return docId, err
				}
				file = append(file, []byte("___eru___")...)
				file = append(file, fileKey...)
			} else {
				finalFileName = fmt.Sprintf("%s_%s", docId, header.Filename)
			}

			reader := bytes.NewReader(file)

			blobClient := containerClient.NewBlockBlobClient(finalFileName)
			_, err = blobClient.UploadStream(ctx, reader, nil)
			if err != nil {
				slog.Info("Failed To Upload File")
				logs.WithContext(ctx).Debug("Failed To Upload File")
				return docId, err
			} else {
				slog.Info("File Uploaded Successfully")
				logs.WithContext(ctx).Debug("File Uploaded Successfully")
				return docId, err
			}
		} else {
			logs.WithContext(ctx).Debug(fmt.Sprintf("Container is not present %s , error: %s", containerName, "Failed to create new container"))
			return docId, err
		}

	}
	return docId, err
}
func (azureStorage *AzureStorage) DownloadFile(ctx context.Context, folderPath string, fileName string, keyName eruaes.AesKey) (file []byte, err error) {
	slog.Info("Downloading File")

	connectionString := azureStorage.Authentication
	containerName := folderPath

	ctx = context.TODO()
	logs.WithContext(ctx).Debug("DownFileB64 - Start")

	serviceClient := azureStorage.getServiceClient(connectionString)

	container_client := serviceClient.ServiceClient().NewContainerClient(containerName)
	blob_name := fileName

	fmt.Println("Blob Name", blob_name)
	blb_client := container_client.NewBlobClient(blob_name)
	fmt.Println(blb_client.URL())

	downloadResponse, err := blb_client.DownloadStream(ctx, nil)
	if err != nil {
		logs.WithContext(ctx).Debug(err.Error())
		return
	}

	byteContainer, err := io.ReadAll(downloadResponse.Body)
	fmt.Println("Downloaded Byte Container", string(byteContainer))
	if err != nil {
		logs.WithContext(ctx).Debug(err.Error())
		return
	}
	if azureStorage.EncryptFiles {
		fmt.Println("Inside Azure Decrypting ")
		var byteContainerKey []byte
		byteContainerSlice := bytes.Split(byteContainer, []byte("___eru___"))
		fmt.Println("Byte Container Slice", len(byteContainerSlice))
		if len(byteContainerSlice) > 1 {
			byteContainerKey = byteContainerSlice[1]
			fmt.Println("Byte Container Key", string(byteContainerKey))
		}

		byteContainer = byteContainerSlice[0]
		byteContainerKey, err = azureStorage.decrypt(ctx, byteContainer, byteContainerKey, keyName)
		if err != nil {
			logs.WithContext(ctx).Debug("Failed while decrypting file")
			return
		}
	}

	logs.WithContext(ctx).Debug("File Downloaded successfully")
	slog.Info("File downloaded successfully and returned as base64", slog.String("file_name", fileName))
	return byteContainer, err
}

func (azureStorage *AzureStorage) MakeFromJson(ctx context.Context, rj *json.RawMessage) error {

	err := json.Unmarshal(*rj, &azureStorage)
	if err != nil {
		logs.WithContext(ctx).Error(err.Error())
		return err
	}
	return nil

}

func (azureStorage *AzureStorage) DeleteStorage(ctx context.Context, forceDelete bool, cloneStorage StorageI) (err error) {
	connectionString := azureStorage.Authentication
	containerName := azureStorage.BucketName
	ctx = context.TODO()
	logs.WithContext(ctx).Debug("Delete Conatiner - Start")

	serviceClient := azureStorage.getServiceClient(connectionString)
	containerClient := serviceClient.ServiceClient().NewContainerClient(containerName)
	_, err = containerClient.Delete(ctx, nil)
	if err != nil {
		logs.WithContext(ctx).Debug("Conatiner deleted successfully")
		return
	} else {
		logs.WithContext(ctx).Debug("Failed to delete container")
		return err

	}
}

func (azureStorage *AzureStorage) CreateStorage(ctx context.Context, cloneStorage StorageI, persist bool) (err error) {
	logs.WithContext(ctx).Info("creating Azure session")
	err = cloneStorage.Init(ctx)
	if err != nil {
		return
	}
	session, err := cloneStorage.GetAttribute("session")
	if err != nil {
		return
	}
	azureStorage.session = session.(azcore.TokenCredential)

	subscriptionId, err := cloneStorage.GetAttribute("subscription_id")
	if err != nil {
		return
	}
	resourceGroup, err := cloneStorage.GetAttribute("resource_group")
	if err != nil {
		return
	}

	if persist {
		be := false
		be, err = cloneStorage.BucketExists(ctx)
		if err != nil {
			return
		}

		logs.WithContext(ctx).Info(fmt.Sprint("be = ", be))

		if !be {
			bn, _ := cloneStorage.GetAttribute("bucket_name")
			logs.WithContext(ctx).Info(bn.(string))

			storageClient, err := armstorage.NewAccountsClient(subscriptionId.(string), azureStorage.session, nil)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprint("Failed to create storage client", err.Error()))
				return err
			}

			skuName := armstorage.SKUNameStandardLRS
			kind := armstorage.KindStorageV2
			accessTier := armstorage.AccessTierHot
			location := azureStorage.Region

			pollerResp, err := storageClient.BeginCreate(
				ctx,
				resourceGroup.(string),
				azureStorage.BucketName,
				armstorage.AccountCreateParameters{
					Location: &location,
					SKU: &armstorage.SKU{
						Name: &skuName,
					},
					Kind: &kind,
					Properties: &armstorage.AccountPropertiesCreateParameters{
						AccessTier: &accessTier,
					},
				},
				nil,
			)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprintf("Failed to start storage account creation: %v", err))
				return err
			}

			_, err = pollerResp.PollUntilDone(ctx, nil)
			if err != nil {
				logs.WithContext(ctx).Error(fmt.Sprintf("Failed to complete storage account creation: %v", err))
				return err
			}
		} else {
			logs.WithContext(ctx).Info("skipping container creation in Azure as it already exists")
		}
	}
	return
}

func (azureStorage *AzureStorage) BucketExists(ctx context.Context) (exists bool, err error) {
	if azureStorage.session == nil {
		logs.WithContext(ctx).Info("creating Azure session")
		err = azureStorage.Init(ctx)
		if err != nil {
			return false, err
		}
	}

	storageClient, err := armstorage.NewAccountsClient(azureStorage.SubscriptionId, azureStorage.session, nil)
	if err != nil {
		logs.WithContext(ctx).Error(fmt.Sprintf("Failed to create storage client: %v", err))
		return false, err
	}

	_, err = storageClient.GetProperties(ctx, azureStorage.ResourceGroup, azureStorage.BucketName, nil)
	if err != nil {
		logs.WithContext(ctx).Info(fmt.Sprintf("Storage account %s does not exist: %v", azureStorage.BucketName, err))
		return false, nil
	}

	logs.WithContext(ctx).Info(fmt.Sprintf("Storage account %s exists", azureStorage.BucketName))
	return true, nil
}

func (azureStorage *AzureStorage) DeleteFile(ctx context.Context, file []byte, fileName string, docType string, folderPath string, keyName eruaes.AesKey) (docId string, err error) {

	connectionString := azureStorage.Authentication
	containerName := azureStorage.BucketName

	ctx = context.TODO()
	logs.WithContext(ctx).Debug("UploadFileB64 - Start")

	serviceClient := azureStorage.getServiceClient(connectionString)

	isContainerExist, err := azureStorage.BucketExists(ctx)
	if isContainerExist == true {

		containerClient := serviceClient.ServiceClient().NewContainerClient(containerName)

		blobClient := containerClient.NewBlockBlobClient(fileName)
		_, err = blobClient.GetProperties(ctx, nil)
		if err != nil {
			_, err = blobClient.Delete(ctx, nil)
			if err != nil {
				logs.WithContext(ctx).Debug("Failed To Delete File")
				return
			} else {
				logs.WithContext(ctx).Debug("File Deleted Successfully")
			}
			return
		} else {
			logs.WithContext(ctx).Debug("File is not present")
		}

	} else {
		logs.WithContext(ctx).Debug("Container is not present")
		return
	}
	return

}

func (azureStorage AzureStorage) encrypt(ctx context.Context, byteContainer []byte, keyName eruaes.AesKey) (eByteContainer []byte, eByteContainerKey []byte, err error) {
	logs.WithContext(ctx).Debug("encrypt - Start")
	if azureStorage.KeyPair != "" {
		byteContainer = eruaes.Pad(byteContainer, 16)
		eByteContainer, err = eruaes.EncryptCBC(ctx, byteContainer, keyName.Key, keyName.Vector)
		if err != nil {
			return
		}
	} else if azureStorage.KmsKey != nil {
		/*aKey := eruaes.AesKey{}
		aKey, err = eruaes.GenerateKey(ctx, 16)
		if err != nil {
			return
		}

		byteContainer = eruaes.Pad(byteContainer, 16)
		eByteContainer, err = eruaes.EncryptCBC(ctx, byteContainer, aKey.Key, []byte(iv))
		if err != nil {
			return
		}

		 eByteContainerKey, err = azureStorage.KmsKey.Encrypt(ctx, aKey.Key)
		if err != nil {
			return
		} */
	} else {
		err = errors.New("encryption key not found")
		return
	}
	return

}

func (azureStorage AzureStorage) decrypt(ctx context.Context, eByteContainer []byte, byteContainerKey []byte, keyName eruaes.AesKey) (byteContainer []byte, err error) {
	if azureStorage.KeyPair != "" {
		byteContainer, err = eruaes.DecryptCBC(ctx, eByteContainer, keyName.Key, keyName.Vector)
		if err != nil {
			return
		}
		byteContainer, err = eruaes.Unpad(byteContainer)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return
		}
	} else if azureStorage.KmsKey != nil {
		byteContainerKey, err = azureStorage.KmsKey.Decrypt(ctx, byteContainerKey)
		if err != nil {
			return
		}

		byteContainer, err = eruaes.DecryptCBC(ctx, eByteContainer, byteContainerKey, []byte(iv))
		if err != nil {
			return
		}
		byteContainer, err = eruaes.Unpad(byteContainer)
		if err != nil {
			logs.WithContext(ctx).Error(err.Error())
			return
		}

	} else {
		err = errors.New("decryption key not found")
		return
	}
	return
}
