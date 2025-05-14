package function

import (
	"context"
	"fmt"
	"io"

	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func S3FileExists(fileName string) (bool, error) {
	// Создаем новый клиент S3
	minioClient, err := minio.New(config.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.Key, config.S3.Secret, ""),
		Secure: true,
	})
	if err != nil {
		ErrLog.Printf("Failed to create S3 client: %v", err)
		return false, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Проверяем метаданные объекта
	_, err = minioClient.StatObject(context.Background(), config.S3.Bucket, fileName, minio.StatObjectOptions{})
	if err != nil {
		ErrLog.Printf("Error checking file existence: %v", err)
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil // Файл не существует
		}
		return false, fmt.Errorf("failed to get object info from S3 file not found: %w", err)
	}

	return true, nil // Файл существует
}

// S3GetSize получает размер файла в S3
func S3GetSize(fileName string) (int64, error) {
	// Создаем новый клиент S3
	minioClient, err := minio.New(config.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.Key, config.S3.Secret, ""),
		Secure: true,
	})
	if err != nil {
		ErrLog.Printf("Failed to create S3 client: %v", err)
		return 0, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Получаем метаданные объекта
	objectInfo, err := minioClient.StatObject(context.Background(), config.S3.Bucket, fileName, minio.StatObjectOptions{})
	if err != nil {
		ErrLog.Printf("Failed to get object info from S3: %v", err)
		return 0, fmt.Errorf("failed to get object info from S3: %w", err)
	}

	return objectInfo.Size, nil // Возвращаем размер объекта
}

// Возвращает объект из S3 в виде io.ReadCloser
func GetFromS3(fileName string) (io.ReadCloser, error) {
	minioClient, err := minio.New(config.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.Key, config.S3.Secret, ""),
		Secure: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	s3Path := fmt.Sprintf("%s/%s", config.S3.BucketSubDir, fileName)

	object, err := minioClient.GetObject(context.Background(), config.S3.Bucket, s3Path, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}

	return object, nil
}

func UploadToS3(fileName string, fileData io.Reader) error {
	// Создаем новый клиент S3
	minioClient, err := minio.New(config.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.Key, config.S3.Secret, ""),
		Secure: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Указываем путь к поддиректории
	s3Path := fmt.Sprintf("%s/%s", config.S3.BucketSubDir, fileName)

	// Загружаем ZIP файл в S3
	_, err = minioClient.PutObject(context.Background(), config.S3.Bucket, s3Path, fileData, -1, minio.PutObjectOptions{
		ContentType: "application/zip",
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return nil
}

func DeleteFromS3(fileName string) error {
	// Создаем новый клиент S3
	minioClient, err := minio.New(config.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.Key, config.S3.Secret, ""),
		Secure: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Указываем путь к поддиректории
	s3Path := fmt.Sprintf("%s/%s", config.S3.BucketSubDir, fileName)

	// Проверяем существование файла
	_, err = minioClient.StatObject(context.Background(), config.S3.Bucket, s3Path, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return fmt.Errorf("file does not exist: %s", s3Path)
		}
		return fmt.Errorf("failed to check file existence: %w", err)
	}

	// Удаляем файл из S3
	err = minioClient.RemoveObject(context.Background(), config.S3.Bucket, s3Path, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file from S3: %w", err)
	}

	return nil
}
