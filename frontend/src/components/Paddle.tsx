
import styled from 'styled-components';
import { Paddle as PaddleType } from '../types/game';

// Google Colors
const PADDLE_COLOR = '#34A853'; // Green

interface PaddleProps {
  $paddleData: PaddleType;
}

const PaddleComponent = styled.div<PaddleProps>`
  position: absolute;
  background-color: ${PADDLE_COLOR};
  width: ${(props) => props.$paddleData.width}px;
  height: ${(props) => props.$paddleData.height}px;
  left: ${(props) => props.$paddleData.x}px;
  top: ${(props) => props.$paddleData.y}px;
  box-shadow: 0 0 5px ${PADDLE_COLOR};
`;

export default PaddleComponent;