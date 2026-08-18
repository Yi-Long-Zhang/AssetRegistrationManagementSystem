package service

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (s *BackupService) createArchive(path, snapshot string) (BackupManifest, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return BackupManifest{}, err
	}
	writer := zip.NewWriter(file)
	manifest := BackupManifest{Version: 1, CreatedAt: time.Now().UTC(), Contents: []BackupContent{}}
	add := func(source, archivePath string) error {
		content, err := addFileToZip(writer, source, archivePath)
		if err == nil {
			manifest.Contents = append(manifest.Contents, content)
		}
		return err
	}
	if err := add(snapshot, "database/assets.db"); err != nil {
		writer.Close()
		file.Close()
		return BackupManifest{}, err
	}
	for _, item := range []struct {
		source string
		prefix string
	}{
		{s.AttachmentDir, "attachments"},
		{s.ArchiveDir, "ticket-archives"},
	} {
		if err := addDirectoryToZip(writer, item.source, item.prefix, add); err != nil {
			writer.Close()
			file.Close()
			return BackupManifest{}, err
		}
	}
	if s.ConfigPath != "" {
		if _, err := os.Stat(s.ConfigPath); err == nil {
			if err := add(s.ConfigPath, "config/config.yaml"); err != nil {
				writer.Close()
				file.Close()
				return BackupManifest{}, err
			}
		}
	}
	rawManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err == nil {
		var entry io.Writer
		entry, err = writer.Create("manifest.json")
		if err == nil {
			_, err = entry.Write(rawManifest)
		}
	}
	closeErr := writer.Close()
	fileErr := file.Close()
	if err != nil {
		return BackupManifest{}, err
	}
	if closeErr != nil {
		return BackupManifest{}, closeErr
	}
	if fileErr != nil {
		return BackupManifest{}, fileErr
	}
	return manifest, nil
}

func addDirectoryToZip(writer *zip.Writer, source, prefix string, add func(string, string) error) error {
	if source == "" {
		return nil
	}
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		return add(path, filepath.ToSlash(filepath.Join(prefix, relative)))
	})
}

func addFileToZip(writer *zip.Writer, source, archivePath string) (BackupContent, error) {
	file, err := os.Open(source)
	if err != nil {
		return BackupContent{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return BackupContent{}, err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return BackupContent{}, err
	}
	header.Name = archivePath
	header.Method = zip.Deflate
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return BackupContent{}, err
	}
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(entry, hash), file)
	if err != nil {
		return BackupContent{}, err
	}
	return BackupContent{Path: archivePath, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func encryptBackupFile(source, target, keyText string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		targetFile.Close()
		if !success {
			_ = os.Remove(target)
		}
	}()
	block, err := aes.NewCipher(encryptionKey(keyText))
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if _, err := targetFile.Write([]byte(backupHeader)); err != nil {
		return err
	}
	buffer := make([]byte, backupChunkSize)
	for {
		count, readErr := sourceFile.Read(buffer)
		if count > 0 {
			nonce := make([]byte, gcm.NonceSize())
			if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
				return err
			}
			sealed := gcm.Seal(nil, nonce, buffer[:count], nil)
			if err := binary.Write(targetFile, binary.BigEndian, uint32(len(sealed))); err != nil {
				return err
			}
			if _, err := targetFile.Write(nonce); err != nil {
				return err
			}
			if _, err := targetFile.Write(sealed); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := targetFile.Sync(); err != nil {
		return err
	}
	success = true
	return nil
}

func decryptBackupFile(source, target, keyText string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	header := make([]byte, len(backupHeader))
	if _, err := io.ReadFull(sourceFile, header); err != nil || string(header) != backupHeader {
		return errors.New("invalid encrypted backup header")
	}
	block, err := aes.NewCipher(encryptionKey(keyText))
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		targetFile.Close()
		if !success {
			_ = os.Remove(target)
		}
	}()
	for {
		var size uint32
		if err := binary.Read(sourceFile, binary.BigEndian, &size); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if size == 0 || size > backupChunkSize+1024 {
			return errors.New("invalid encrypted backup chunk")
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(sourceFile, nonce); err != nil {
			return err
		}
		sealed := make([]byte, size)
		if _, err := io.ReadFull(sourceFile, sealed); err != nil {
			return err
		}
		plain, err := gcm.Open(nil, nonce, sealed, nil)
		if err != nil {
			return errors.New("backup decryption or integrity verification failed")
		}
		if _, err := targetFile.Write(plain); err != nil {
			return err
		}
	}
	if err := targetFile.Sync(); err != nil {
		return err
	}
	success = true
	return nil
}
